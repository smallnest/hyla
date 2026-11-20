package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"hyla/internal/plugins/manifest"
)

func TestExpectedAssets(t *testing.T) {
	platforms, err := parsePlatforms(defaultPlatforms)
	if err != nil {
		t.Fatal(err)
	}
	targets := []pluginTarget{
		{
			Name:     "timeline",
			Manifest: &manifest.Manifest{ID: "hyla.timeline", Version: "1.0.0"},
			IsGo:     true,
		},
		{
			Name:     "nord-theme",
			Manifest: &manifest.Manifest{ID: "hyla.nord", Version: "0.2.0"},
			IsGo:     false,
		},
	}

	got := expectedAssets(targets, platforms)
	// 6 platforms * 2 files (.hylapkg + .sha256) + 1 declarative * 2 files = 14
	if len(got) != 14 {
		t.Fatalf("got %d assets, want 14: %v", len(got), got)
	}
	for _, want := range []string{
		"hyla.timeline-1.0.0-darwin-arm64.hylapkg",
		"hyla.timeline-1.0.0-darwin-arm64.hylapkg.sha256",
		"hyla.timeline-1.0.0-windows-amd64.hylapkg",
		"hyla.nord-0.2.0.hylapkg",
		"hyla.nord-0.2.0.hylapkg.sha256",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("missing asset %q in %v", want, got)
		}
	}
}

func TestStageRuntimeFilesSkipsGoAndLocalBins(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "stage")

	writeFile := func(rel, content string) {
		t.Helper()
		p := filepath.Join(src, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writeFile("hyla-plugin.toml", "api_version = 1\nid = \"org.example.demo\"\nname = \"Demo\"\nversion = \"1.0.0\"\n")
	writeFile("README.md", "# Demo\n")
	writeFile("web/index.html", "<html></html>\n")
	writeFile("go.mod", "module example.com/demo\n\ngo 1.24\n")
	writeFile("go.sum", "")
	writeFile("main.go", "package main\nfunc main() {}\n")
	writeFile("main_test.go", "package main\n")
	writeFile("bin/old-local-bin", "stale")
	writeFile("old.hylapkg", "zip")

	if err := stageRuntimeFiles(src, dst); err != nil {
		t.Fatal(err)
	}

	for _, keep := range []string{"hyla-plugin.toml", "README.md", "web/index.html"} {
		if _, err := os.Stat(filepath.Join(dst, filepath.FromSlash(keep))); err != nil {
			t.Errorf("expected staged file %s: %v", keep, err)
		}
	}
	for _, skip := range []string{"go.mod", "go.sum", "main.go", "main_test.go", "bin/old-local-bin", "old.hylapkg"} {
		if _, err := os.Stat(filepath.Join(dst, filepath.FromSlash(skip))); err == nil {
			t.Errorf("expected %s to be excluded from stage", skip)
		}
	}
}

func TestBuildAndPackGoPlugin(t *testing.T) {
	pluginsDir := t.TempDir()
	pdir := filepath.Join(pluginsDir, "sample")
	if err := os.MkdirAll(filepath.Join(pdir, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifestTOML := `api_version = 1
id = "org.example.sample"
name = "Sample"
version = "0.1.0"

[permissions]
scopes = ["ui.webview", "ui.slot"]

[app]
command = ["./bin/sample-plugin"]
transport = "stdio"

[[ui.slots]]
id = "main"
mount = "pane"
title = "Sample"
mode = "webview"
web_root = "web"
entry = "index.html"
`
	if err := os.WriteFile(filepath.Join(pdir, "hyla-plugin.toml"), []byte(manifestTOML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "web", "index.html"), []byte("<h1>ok</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "go.mod"), []byte("module example.com/sample\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	targets, err := discoverPlugins(pluginsDir)
	if err != nil {
		t.Fatal(err)
	}
	platforms, err := parsePlatforms("linux/amd64,windows/amd64")
	if err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(t.TempDir(), "out")
	if err := buildPlugins(targets, platforms, outDir); err != nil {
		t.Fatal(err)
	}

	linuxPkg := filepath.Join(outDir, "org.example.sample-0.1.0-linux-amd64.hylapkg")
	winPkg := filepath.Join(outDir, "org.example.sample-0.1.0-windows-amd64.hylapkg")
	for _, f := range []string{linuxPkg, linuxPkg + ".sha256", winPkg, winPkg + ".sha256"} {
		if st, err := os.Stat(f); err != nil || st.Size() == 0 {
			t.Fatalf("expected output %s: err=%v", f, err)
		}
	}

	checkZip := func(pkg, wantBin string) {
		t.Helper()
		zr, err := zip.OpenReader(pkg)
		if err != nil {
			t.Fatal(err)
		}
		defer zr.Close()
		var names []string
		for _, f := range zr.File {
			names = append(names, f.Name)
			if strings.HasSuffix(f.Name, ".go") || f.Name == "go.mod" {
				t.Errorf("%s should not include source file %s", pkg, f.Name)
			}
		}
		if !slices.Contains(names, wantBin) {
			t.Errorf("%s missing %s (entries=%v)", pkg, wantBin, names)
		}
		if !slices.Contains(names, "hyla-plugin.toml") || !slices.Contains(names, "web/index.html") {
			t.Errorf("%s missing manifest or web entry (entries=%v)", pkg, names)
		}
	}
	checkZip(linuxPkg, "bin/sample-plugin")
	checkZip(winPkg, "bin/sample-plugin.exe")
}
