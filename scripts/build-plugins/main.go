package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"hyla/internal/plugins/manifest"
)

const defaultPlatforms = "linux/amd64,linux/arm64,darwin/amd64,darwin/arm64,windows/amd64,windows/arm64"

type targetPlatform struct {
	OS   string
	Arch string
}

type binTarget struct {
	RelPath string
	GoPkg   string
}

type pluginTarget struct {
	Name     string
	Dir      string
	Manifest *manifest.Manifest
	Binaries []binTarget
	IsGo     bool
}

func main() {
	pluginsDir := flag.String("plugins-dir", "plugins", "directory containing first-party plugin subdirectories")
	outDir := flag.String("out", "build/plugins", "output directory for packed .hylapkg and .sha256 files")
	platformsFlag := flag.String("platforms", defaultPlatforms, "comma-separated os/arch target platforms")
	listAssetsOnly := flag.Bool("list-assets", false, "print expected release asset filenames and exit")
	flag.Parse()

	platforms, err := parsePlatforms(*platformsFlag)
	if err != nil {
		fail(err)
	}
	targets, err := discoverPlugins(*pluginsDir)
	if err != nil {
		fail(err)
	}
	if len(targets) == 0 {
		fail(fmt.Errorf("no plugins found under %s", *pluginsDir))
	}

	if *listAssetsOnly {
		for _, a := range expectedAssets(targets, platforms) {
			fmt.Println(a)
		}
		return
	}

	if err := buildPlugins(targets, platforms, *outDir); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "build-plugins:", err)
	os.Exit(1)
}

func parsePlatforms(raw string) ([]targetPlatform, error) {
	var out []targetPlatform
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		goos, goarch, ok := strings.Cut(part, "/")
		if !ok || strings.TrimSpace(goos) == "" || strings.TrimSpace(goarch) == "" {
			return nil, fmt.Errorf("invalid platform %q (want os/arch)", part)
		}
		out = append(out, targetPlatform{
			OS:   strings.TrimSpace(goos),
			Arch: strings.TrimSpace(goarch),
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no platforms specified")
	}
	return out, nil
}

func discoverPlugins(pluginsDir string) ([]pluginTarget, error) {
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return nil, fmt.Errorf("read plugins dir %s: %w", pluginsDir, err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	var targets []pluginTarget
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		dir := filepath.Join(pluginsDir, ent.Name())
		manifestPath := filepath.Join(dir, manifest.ManifestFileName)
		if st, err := os.Stat(manifestPath); err != nil || st.IsDir() {
			continue
		}
		m, err := manifest.LoadFile(dir)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", dir, err)
		}
		bins, isGo, err := pluginBinTargets(dir, m)
		if err != nil {
			return nil, fmt.Errorf("inspect %s: %w", dir, err)
		}
		targets = append(targets, pluginTarget{
			Name:     ent.Name(),
			Dir:      dir,
			Manifest: m,
			Binaries: bins,
			IsGo:     isGo,
		})
	}
	return targets, nil
}

func pluginBinTargets(dir string, m *manifest.Manifest) ([]binTarget, bool, error) {
	goModInfo, err := os.Stat(filepath.Join(dir, "go.mod"))
	hasGoMod := err == nil && !goModInfo.IsDir()
	if !hasGoMod {
		return nil, false, nil
	}

	seen := map[string]bool{}
	var bins []binTarget
	for _, spec := range []*manifest.HostProcessSpec{m.App, m.Daemon} {
		if spec == nil || len(spec.Command) == 0 {
			continue
		}
		cmd0 := spec.Command[0]
		if filepath.IsAbs(cmd0) || !strings.ContainsRune(cmd0, '/') {
			continue
		}
		rel, err := manifest.ValidateRelPath(cmd0)
		if err != nil {
			return nil, false, err
		}
		if seen[rel] {
			continue
		}
		seen[rel] = true
		pkg := "."
		if st, err := os.Stat(filepath.Join(dir, "cmd", filepath.Base(rel))); err == nil && st.IsDir() {
			pkg = "./cmd/" + filepath.Base(rel)
		}
		bins = append(bins, binTarget{RelPath: rel, GoPkg: pkg})
	}
	return bins, len(bins) > 0, nil
}

func archiveBaseName(m *manifest.Manifest, plat *targetPlatform) string {
	if plat == nil {
		return fmt.Sprintf("%s-%s.hylapkg", m.ID, m.Version)
	}
	return fmt.Sprintf("%s-%s-%s-%s.hylapkg", m.ID, m.Version, plat.OS, plat.Arch)
}

func expectedAssets(targets []pluginTarget, platforms []targetPlatform) []string {
	var assets []string
	for _, pt := range targets {
		if !pt.IsGo {
			name := archiveBaseName(pt.Manifest, nil)
			assets = append(assets, name, name+".sha256")
			continue
		}
		for i := range platforms {
			name := archiveBaseName(pt.Manifest, &platforms[i])
			assets = append(assets, name, name+".sha256")
		}
	}
	sort.Strings(assets)
	return assets
}

func buildPlugins(targets []pluginTarget, platforms []targetPlatform, outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir %s: %w", outDir, err)
	}
	for _, pt := range targets {
		if !pt.IsGo {
			if err := buildAndPackTarget(pt, nil, outDir); err != nil {
				return err
			}
			continue
		}
		for i := range platforms {
			plat := platforms[i]
			if err := buildAndPackTarget(pt, &plat, outDir); err != nil {
				return err
			}
		}
	}
	return nil
}

func buildAndPackTarget(pt pluginTarget, plat *targetPlatform, outDir string) error {
	workDir, err := os.MkdirTemp("", "hyla-plugin-build-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)

	stageDir := filepath.Join(workDir, "stage")
	if err := stageRuntimeFiles(pt.Dir, stageDir); err != nil {
		return fmt.Errorf("stage %s: %w", pt.Name, err)
	}

	var expectedBins []string
	if pt.IsGo && plat != nil {
		for _, bin := range pt.Binaries {
			relOut := bin.RelPath
			if plat.OS == "windows" && !strings.HasSuffix(strings.ToLower(relOut), ".exe") {
				relOut += ".exe"
			}
			outBin := filepath.Join(stageDir, filepath.FromSlash(relOut))
			if err := os.MkdirAll(filepath.Dir(outBin), 0o755); err != nil {
				return err
			}
			fmt.Printf("Building %s (%s v%s) for %s/%s\n", pt.Name, pt.Manifest.ID, pt.Manifest.Version, plat.OS, plat.Arch)
			cmd := exec.Command("go", "build", "-trimpath", "-ldflags", "-s -w", "-o", outBin, bin.GoPkg)
			cmd.Dir = pt.Dir
			cmd.Env = append(os.Environ(),
				"CGO_ENABLED=0",
				"GOOS="+plat.OS,
				"GOARCH="+plat.Arch,
			)
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("go build %s (%s/%s): %w\n%s", pt.Name, plat.OS, plat.Arch, err, strings.TrimSpace(string(out)))
			}
			if err := os.Chmod(outBin, 0o755); err != nil {
				return err
			}
			expectedBins = append(expectedBins, relOut)
		}
	} else {
		fmt.Printf("Packing %s (%s v%s)\n", pt.Name, pt.Manifest.ID, pt.Manifest.Version)
	}

	archiveName := archiveBaseName(pt.Manifest, plat)
	outArchive := filepath.Join(outDir, archiveName)
	packRes, err := manifest.Pack(stageDir, outArchive)
	if err != nil {
		return fmt.Errorf("pack %s: %w", archiveName, err)
	}

	sumLine := fmt.Sprintf("%s  %s\n", packRes.SHA256, filepath.Base(outArchive))
	if err := os.WriteFile(outArchive+".sha256", []byte(sumLine), 0o644); err != nil {
		return err
	}

	verifyDir := filepath.Join(workDir, "verify")
	store := &manifest.Store{BaseDir: verifyDir}
	inst, err := store.InstallPackage(outArchive, manifest.InstallOptions{
		Enable:   true,
		GrantAll: true,
	})
	if err != nil {
		return fmt.Errorf("verify install %s: %w", archiveName, err)
	}
	for _, relBin := range expectedBins {
		installedBin := filepath.Join(inst.RootDir, filepath.FromSlash(relBin))
		st, err := os.Stat(installedBin)
		if err != nil || st.IsDir() || st.Size() == 0 {
			return fmt.Errorf("verify %s: missing or empty binary %s", archiveName, relBin)
		}
	}
	fmt.Printf("Packed %s (%d files, %d bytes, sha256=%s)\n", archiveName, packRes.Files, packRes.Bytes, packRes.SHA256[:12])
	return nil
}

func stageRuntimeFiles(srcDir, dstDir string) error {
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}
	return filepath.WalkDir(srcDir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relOS, err := filepath.Rel(srcDir, p)
		if err != nil {
			return err
		}
		if relOS == "." {
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are forbidden in plugins: %s", relOS)
		}

		relSlash := filepath.ToSlash(relOS)
		name := d.Name()
		if d.IsDir() {
			if relSlash == "bin" || name == ".git" || name == "node_modules" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dstDir, relOS), 0o755)
		}

		if shouldSkipRuntimeFile(name) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return copyFile(p, filepath.Join(dstDir, relOS), info.Mode().Perm())
	})
}

func shouldSkipRuntimeFile(name string) bool {
	switch name {
	case ".DS_Store", "go.mod", "go.sum", "go.work", "go.work.sum":
		return true
	}
	return strings.HasSuffix(name, ".go") ||
		strings.HasSuffix(name, ".hylapkg") ||
		strings.HasSuffix(name, ".hylapkg.sha256")
}

func copyFile(src, dst string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if mode == 0 {
		mode = 0o644
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
