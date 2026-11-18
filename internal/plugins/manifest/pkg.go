package manifest

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	MaxPackageFiles             = 4096
	MaxPackageSingleFileBytes   = 64 << 20  // 64 MiB
	MaxPackageUncompressedBytes = 128 << 20 // 128 MiB
)

type InstallOptions struct {
	Enable           bool
	GrantAll         bool
	GrantPermissions []string
	CurrentVersion   string // optional Hyla version check against [engines].hyla
}

type PackResult struct {
	ArchivePath string
	SHA256      string
	Files       int
	Bytes       int64
	Manifest    *Manifest
}

// InstallPackage safely extracts and installs a .hylapkg archive into the store.
func (s *Store) InstallPackage(pkgPath string, opts InstallOptions) (*InstalledPlugin, error) {
	sum, err := sha256File(pkgPath)
	if err != nil {
		return nil, err
	}
	zr, err := zip.OpenReader(pkgPath)
	if err != nil {
		return nil, fmt.Errorf("open package %s: %w", pkgPath, err)
	}
	defer zr.Close()

	if len(zr.File) == 0 {
		return nil, fmt.Errorf("package %s is empty", pkgPath)
	}
	if len(zr.File) > MaxPackageFiles {
		return nil, fmt.Errorf("package %s has %d entries (max %d)", pkgPath, len(zr.File), MaxPackageFiles)
	}

	stripPrefix, err := detectZipRootPrefix(zr.File)
	if err != nil {
		return nil, fmt.Errorf("package %s: %w", pkgPath, err)
	}

	if err := os.MkdirAll(s.PluginsDir(), 0o755); err != nil {
		return nil, err
	}
	stageDir, err := os.MkdirTemp(s.PluginsDir(), ".staging-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stageDir)

	var totalBytes int64
	for _, f := range zr.File {
		rel, skip, err := sanitizeZipEntry(f.Name, stripPrefix)
		if err != nil {
			return nil, fmt.Errorf("package %s entry %q: %w", pkgPath, f.Name, err)
		}
		if f.Mode()&fs.ModeSymlink != 0 {
			return nil, fmt.Errorf("package %s contains forbidden symlink %q", pkgPath, f.Name)
		}
		if skip {
			continue
		}
		dst := filepath.Join(stageDir, filepath.FromSlash(rel))
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return nil, err
			}
			continue
		}
		if f.UncompressedSize64 > MaxPackageSingleFileBytes {
			return nil, fmt.Errorf("package %s entry %q exceeds single-file limit (%d bytes)", pkgPath, rel, MaxPackageSingleFileBytes)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return nil, err
		}
		mode := fs.FileMode(0o644)
		if f.Mode()&0o111 != 0 {
			mode = 0o755
		}
		n, err := extractZipFile(f, dst, mode, MaxPackageUncompressedBytes-totalBytes)
		if err != nil {
			return nil, fmt.Errorf("extract %s: %w", rel, err)
		}
		totalBytes += n
		if totalBytes > MaxPackageUncompressedBytes {
			return nil, fmt.Errorf("package %s exceeds uncompressed limit (%d bytes)", pkgPath, MaxPackageUncompressedBytes)
		}
	}

	m, err := LoadFile(stageDir)
	if err != nil {
		return nil, err
	}
	if isBuiltinEntry(m.ID) {
		return nil, fmt.Errorf("cannot install package %q: conflicts with built-in plugin", m.ID)
	}
	if opts.CurrentVersion != "" {
		if err := m.CompatibleWith(opts.CurrentVersion); err != nil {
			return nil, err
		}
	}
	if err := ensureNoSymlinks(stageDir); err != nil {
		return nil, err
	}

	targetDir, err := s.PluginDir(m.ID)
	if err != nil {
		return nil, err
	}
	if err := swapDirectoryAtomic(stageDir, targetDir); err != nil {
		return nil, err
	}

	now := time.Now().UnixMilli()
	var granted []string
	if opts.GrantAll {
		granted = append([]string(nil), m.Permissions.Scopes...)
		slices.Sort(granted)
	} else if len(opts.GrantPermissions) > 0 {
		granted = intersection(opts.GrantPermissions, m.Permissions.Scopes)
	}

	registryMu.Lock()
	defer registryMu.Unlock()
	reg, err := s.loadRegistryLocked()
	if err != nil {
		return nil, err
	}
	entry := RegistryEntry{
		ID:                 m.ID,
		Enabled:            opts.Enable,
		Version:            m.Version,
		Source:             "package",
		SHA256:             sum,
		PendingInstall:     false,
		GrantedPermissions: granted,
		InstalledAt:        now,
		UpdatedAt:          now,
	}
	if prev, ok := reg.Plugins[m.ID]; ok {
		if prev.InstalledAt != 0 {
			entry.InstalledAt = prev.InstalledAt
		}
		if !opts.GrantAll && len(opts.GrantPermissions) == 0 && len(prev.GrantedPermissions) > 0 {
			entry.GrantedPermissions = intersection(prev.GrantedPermissions, m.Permissions.Scopes)
		}
	}
	reg.Plugins[m.ID] = entry
	if err := s.saveRegistryLocked(reg); err != nil {
		return nil, err
	}
	return &InstalledPlugin{Entry: entry, Manifest: m, RootDir: targetDir}, nil
}

// Pack validates srcDir and bundles it into a .hylapkg archive at outPath
// (defaulting to "<id>-<version>.hylapkg" in the current directory if outPath == "").
func Pack(srcDir, outPath string) (*PackResult, error) {
	absSrc, err := filepath.Abs(srcDir)
	if err != nil {
		return nil, err
	}
	if err := ensureNoSymlinks(absSrc); err != nil {
		return nil, err
	}
	m, err := LoadFile(absSrc)
	if err != nil {
		return nil, err
	}
	if outPath == "" {
		outPath = fmt.Sprintf("%s-%s.hylapkg", m.ID, m.Version)
	}
	absOut, err := filepath.Abs(outPath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(absOut), 0o755); err != nil {
		return nil, err
	}

	tmp, err := os.CreateTemp(filepath.Dir(absOut), ".pack-*")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	zw := zip.NewWriter(tmp)
	var files int
	var totalBytes int64
	err = filepath.WalkDir(absSrc, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if p == absSrc {
			return nil
		}
		// Do not bundle the destination archive itself if outPath sits inside srcDir.
		if p == absOut {
			return nil
		}
		name := d.Name()
		if name == ".git" || name == ".DS_Store" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("cannot pack symlink %s", p)
		}
		relOS, err := filepath.Rel(absSrc, p)
		if err != nil {
			return err
		}
		rel := filepath.ToSlash(relOS)
		if d.IsDir() {
			return nil
		}
		files++
		if files > MaxPackageFiles {
			return fmt.Errorf("package exceeds maximum file count (%d)", MaxPackageFiles)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > MaxPackageSingleFileBytes {
			return fmt.Errorf("%s exceeds single file size limit (%d bytes)", rel, MaxPackageSingleFileBytes)
		}
		totalBytes += info.Size()
		if totalBytes > MaxPackageUncompressedBytes {
			return fmt.Errorf("total package size exceeds limit (%d bytes)", MaxPackageUncompressedBytes)
		}
		hdr, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		hdr.Name = rel
		hdr.Method = zip.Deflate
		hdr.SetMode(info.Mode())
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(w, f)
		_ = f.Close()
		return copyErr
	})
	if err != nil {
		_ = zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	if err := os.Rename(tmpPath, absOut); err != nil {
		return nil, err
	}
	cleanup = false
	sum, err := sha256File(absOut)
	if err != nil {
		return nil, err
	}
	return &PackResult{
		ArchivePath: absOut,
		SHA256:      sum,
		Files:       files,
		Bytes:       totalBytes,
		Manifest:    m,
	}, nil
}

func detectZipRootPrefix(entries []*zip.File) (string, error) {
	hasRootManifest := false
	topDirs := map[string]bool{}
	for _, f := range entries {
		clean, skip, err := sanitizeZipEntry(f.Name, "")
		if err != nil {
			return "", err
		}
		if skip {
			continue
		}
		if clean == ManifestFileName && !f.FileInfo().IsDir() {
			hasRootManifest = true
		}
		if idx := strings.IndexByte(clean, '/'); idx >= 0 {
			topDirs[clean[:idx]] = true
		} else if f.FileInfo().IsDir() {
			topDirs[clean] = true
		} else {
			topDirs[""] = true
		}
	}
	if hasRootManifest {
		return "", nil
	}
	if len(topDirs) == 1 {
		for dir := range topDirs {
			if dir != "" {
				cand := dir + "/" + ManifestFileName
				for _, f := range entries {
					clean, _, _ := sanitizeZipEntry(f.Name, "")
					if clean == cand && !f.FileInfo().IsDir() {
						return dir + "/", nil
					}
				}
			}
		}
	}
	return "", fmt.Errorf("missing %s", ManifestFileName)
}

func sanitizeZipEntry(rawName, stripPrefix string) (rel string, skip bool, err error) {
	if rawName == "" {
		return "", true, nil
	}
	if strings.ContainsRune(rawName, 0) || strings.Contains(rawName, `\`) {
		return "", false, fmt.Errorf("illegal character in path")
	}
	if strings.HasPrefix(rawName, "/") || (len(rawName) >= 2 && rawName[1] == ':') {
		return "", false, fmt.Errorf("absolute path not allowed")
	}
	isDir := strings.HasSuffix(rawName, "/")
	cleaned := path.Clean(strings.TrimSuffix(rawName, "/"))
	if cleaned == "." {
		return "", true, nil
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") {
		return "", false, fmt.Errorf("path traversal not allowed")
	}
	for _, seg := range strings.Split(cleaned, "/") {
		if seg == ".." || seg == "" {
			return "", false, fmt.Errorf("invalid path segment")
		}
	}
	if stripPrefix != "" {
		dirOnly := strings.TrimSuffix(stripPrefix, "/")
		if cleaned == dirOnly && isDir {
			return "", true, nil
		}
		if !strings.HasPrefix(cleaned, stripPrefix) {
			return "", false, fmt.Errorf("entry %q outside archive root %q", cleaned, dirOnly)
		}
		cleaned = strings.TrimPrefix(cleaned, stripPrefix)
		if cleaned == "" || cleaned == "." {
			return "", true, nil
		}
	}
	return cleaned, false, nil
}

func extractZipFile(f *zip.File, dst string, mode fs.FileMode, remaining int64) (int64, error) {
	if remaining < 0 {
		remaining = 0
	}
	rc, err := f.Open()
	if err != nil {
		return 0, err
	}
	defer rc.Close()

	capRead := min(remaining, int64(MaxPackageSingleFileBytes))
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(out, io.LimitReader(rc, capRead+1))
	closeErr := out.Close()
	if err != nil {
		return n, err
	}
	if closeErr != nil {
		return n, closeErr
	}
	if n > capRead {
		return n, fmt.Errorf("uncompressed data exceeds limit")
	}
	if err := os.Chmod(dst, mode); err != nil {
		return n, err
	}
	return n, nil
}

func ensureNoSymlinks(root string) error {
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are forbidden in plugins: %s", p)
		}
		info, lerr := os.Lstat(p)
		if lerr != nil {
			return lerr
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are forbidden in plugins: %s", p)
		}
		return nil
	})
}

func swapDirectoryAtomic(stageDir, targetDir string) error {
	backupDir := targetDir + ".bak"
	_ = os.RemoveAll(backupDir)
	hadExisting := false
	if _, err := os.Lstat(targetDir); err == nil {
		if err := os.Rename(targetDir, backupDir); err != nil {
			return err
		}
		hadExisting = true
	}
	if err := os.Rename(stageDir, targetDir); err != nil {
		if hadExisting {
			_ = os.Rename(backupDir, targetDir)
		}
		return err
	}
	if hadExisting {
		_ = os.RemoveAll(backupDir)
	}
	return nil
}

func sha256File(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
