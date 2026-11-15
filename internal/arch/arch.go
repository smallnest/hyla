// Package arch holds the rules that keep Hyla's packages in layers, so
// that its root, one package main, can move into internal/ a piece at a
// time without a piece taking another with it.
//
// Each layer names the packages it may import, and the rule is one
// direction: a package imports only from its own layer's list and
// below it. The layers begin with the packages that import nothing of
// Hyla's and rise with what runs on them, to hylad and the hyla CLI, and
// at the top the root package main, which imports every layer.
//
// A list names every package of its layer and below that the packages
// there import, so a package that moves down a layer leaves the list
// that names it stale, and one that imports above itself fails.
package arch

import (
	"slices"
	"strings"
)

// Layer is a rank of packages, from the bottom up.
type Layer struct {
	// Name names the layer in a failure.
	Name string
	// Package is what a package of the layer may import, as import
	// paths of this module, a path ending in a star matching its
	// prefix. The standard library and the modules others own are
	// outside these rules.
	Package []string
}

// Layers are the layers of Hyla's own packages, from the bottom up.
// They name the packages of internal/ as they stand; a package that
// moves into one goes into its layer's list, and its users keep the
// imports they had.
var Layers = []Layer{
	// The root, package main: it may import every layer, so its list
	// names them all.
	{
		Name: "root",
		Package: []string{
			"*",
		},
	},
	{
		// The bottom: what has no dependence on the others.
		Name: "base",
		Package: []string{
			"hyla/internal/actions",
			"hyla/internal/adblock",
			"hyla/internal/platform",
			"hyla/internal/build",
			"hyla/internal/claude",
			"hyla/internal/clilink",
			"hyla/internal/detach",
			"hyla/internal/detect",
			"hyla/internal/git",
			"hyla/internal/proto",
			"hyla/internal/shell",
			"hyla/internal/update",
		},
	},
	{
		// What the wire and the settings carry, on the base.
		Name: "wire",
		Package: []string{
			"hyla/internal/actions",
			"hyla/internal/adblock",
			"hyla/internal/platform",
			"hyla/internal/build",
			"hyla/internal/claude",
			"hyla/internal/clilink",
			"hyla/internal/detach",
			"hyla/internal/detect",
			"hyla/internal/git",
			"hyla/internal/proto",
			"hyla/internal/remote",
			"hyla/internal/service",
			"hyla/internal/settings",
			"hyla/internal/shell",
			"hyla/internal/update",
		},
	},
	{
		// What hylad runs, and what groups its sessions.
		Name: "engine",
		Package: []string{
			"hyla/internal/actions",
			"hyla/internal/adblock",
			"hyla/internal/platform",
			"hyla/internal/app/magpie",
			"hyla/internal/app/sync",
			"hyla/internal/build",
			"hyla/internal/claude",
			"hyla/internal/clilink",
			"hyla/internal/detach",
			"hyla/internal/detect",
			"hyla/internal/feishu",
			"hyla/internal/git",
			"hyla/internal/pets",
			"hyla/internal/proto",
			"hyla/internal/remote",
			"hyla/internal/service",
			"hyla/internal/settings",
			"hyla/internal/shell",
			"hyla/internal/sync",
			"hyla/internal/update",
		},
	},
	{
		// hylad, the daemon the sessions run in.
		Name: "daemon",
		Package: []string{
			"hyla/internal/actions",
			"hyla/internal/adblock",
			"hyla/internal/platform",
			"hyla/internal/app/filepane",
			"hyla/internal/app/magpie",
			"hyla/internal/app/sync",
			"hyla/internal/build",
			"hyla/internal/claude",
			"hyla/internal/clilink",
			"hyla/internal/client",
			"hyla/internal/detach",
			"hyla/internal/detect",
			"hyla/internal/feishu",
			"hyla/internal/git",
			"hyla/internal/pets",
			"hyla/internal/pocket",
			"hyla/internal/proto",
			"hyla/internal/remote",
			"hyla/internal/service",
			"hyla/internal/settings",
			"hyla/internal/shell",
			"hyla/internal/sync",
			"hyla/internal/update",
		},
	},
	{
		// The views, which draw the app's windows.
		Name: "ui",
		Package: []string{
			"hyla/internal/actions",
			"hyla/internal/adblock",
			"hyla/internal/platform",
			"hyla/internal/app/filepane",
			"hyla/internal/app/magpie",
			"hyla/internal/app/sync",
			"hyla/internal/build",
			"hyla/internal/claude",
			"hyla/internal/clilink",
			"hyla/internal/client",
			"hyla/internal/daemon",
			"hyla/internal/detach",
			"hyla/internal/detect",
			"hyla/internal/feishu",
			"hyla/internal/git",
			"hyla/internal/pets",
			"hyla/internal/pocket",
			"hyla/internal/proto",
			"hyla/internal/remote",
			"hyla/internal/service",
			"hyla/internal/settings",
			"hyla/internal/shell",
			"hyla/internal/sync",
			"hyla/internal/ui/controls",
			"hyla/internal/ui/glyph",
			"hyla/internal/ui/marks",
			"hyla/internal/ui/style",
			"hyla/internal/ui/theme",
			"hyla/internal/ui/wash",
			"hyla/internal/update",
		},
	},
	{
		// The hyla CLI, which drives hylad from a shell or an agent.
		Name: "cli",
		Package: []string{
			"hyla/internal/actions",
			"hyla/internal/adblock",
			"hyla/internal/platform",
			"hyla/internal/app/filepane",
			"hyla/internal/app/magpie",
			"hyla/internal/app/sync",
			"hyla/internal/build",
			"hyla/internal/claude",
			"hyla/internal/cli",
			"hyla/internal/client",
			"hyla/internal/clilink",
			"hyla/internal/daemon",
			"hyla/internal/detach",
			"hyla/internal/detect",
			"hyla/internal/feishu",
			"hyla/internal/git",
			"hyla/internal/pets",
			"hyla/internal/pocket",
			"hyla/internal/proto",
			"hyla/internal/remote",
			"hyla/internal/service",
			"hyla/internal/settings",
			"hyla/internal/shell",
			"hyla/internal/sync",
			"hyla/internal/update",
		},
	},
}

// Module is the import path of Hyla's own code.
const Module = "hyla"

// Imported reports whether an import path is of this module.
func Imported(path string) bool {
	return path == Module || strings.HasPrefix(path, Module+"/")
}

// layerNames are the packages of internal/ and the layer each is of.
var layerNames = map[string]string{
	"hyla/internal/actions":      "base",
	"hyla/internal/app/filepane": "daemon",
	"hyla/internal/app/magpie":   "engine",
	"hyla/internal/app/sync":     "engine",
	"hyla/internal/adblock":      "base",
	"hyla/internal/arch":         "base",
	"hyla/internal/build":        "base",
	"hyla/internal/claude":       "base",
	"hyla/internal/clilink":      "base",
	"hyla/internal/detach":       "base",
	"hyla/internal/detect":       "base",
	"hyla/internal/git":          "base",
	"hyla/internal/platform":     "base",
	"hyla/internal/proto":        "base",
	"hyla/internal/shell":        "base",
	"hyla/internal/update":       "base",
	"hyla/internal/remote":       "wire",
	"hyla/internal/service":      "wire",
	"hyla/internal/settings":     "wire",
	"hyla/internal/feishu":       "engine",
	"hyla/internal/pets":         "engine",
	"hyla/internal/sync":         "engine",
	"hyla/internal/client":       "daemon",
	"hyla/internal/daemon":       "daemon",
	"hyla/internal/pocket":       "daemon",
	"hyla/internal/cli":          "cli",
	"hyla/internal/ui/controls":  "ui",
	"hyla/internal/ui/glyph":     "ui",
	"hyla/internal/ui/marks":     "ui",
	"hyla/internal/ui/style":     "ui",
	"hyla/internal/ui/theme":     "ui",
	"hyla/internal/ui/wash":      "ui",
}

// LayerOf finds the layer of a package of this module.
func LayerOf(pkg string) (Layer, bool) {
	if !Imported(pkg) {
		return Layer{}, false
	}
	// locales, and the like, are the root's: the app and the daemon
	// both read them, and neither is above the other for it.
	if pkg == Module || !strings.HasPrefix(pkg, Module+"/internal/") {
		return Layers[0], true
	}
	// The longest name that holds the package, as one may hold another:
	// "hyla/internal/cli" holds "hyla/internal/cli/agents".
	name := ""
	for p := range layerNames {
		if (pkg == p || strings.HasPrefix(pkg, p+"/")) && len(p) > len(name) {
			name = p
		}
	}
	for _, l := range Layers {
		if l.Name == layerNames[name] {
			return l, true
		}
	}
	// A package internal/ gains, before it is listed, is below every
	// layer, as locales is.
	return Layer{}, false
}

// Allows reports whether a package of layer may import path, which a
// star in its list matches as a prefix.
func (l Layer) Allows(path string) bool {
	for _, p := range l.Package {
		if p == "*" {
			return true
		}
		if strings.HasSuffix(p, "*") {
			if strings.HasPrefix(path, strings.TrimSuffix(p, "*")) {
				return true
			}
		} else if p == path {
			return true
		}
	}
	return false
}

// Check reports the imports of a package of this module that its layer
// does not allow, sorted. An import outside the module is never one: the
// rules are Hyla's own.
func Check(pkg string, imports []string) []string {
	from, ok := LayerOf(pkg)
	if !ok {
		return nil
	}
	var bad []string
	for _, imp := range imports {
		if !Imported(imp) || imp == pkg {
			continue
		}
		// The root is a package of its own: an import of it is a cycle.
		if imp == Module {
			bad = append(bad, imp)
			continue
		}
		if !strings.HasPrefix(imp, Module+"/internal/") {
			continue // locales, and the like, which every layer reads
		}
		to, ok := LayerOf(imp)
		if !ok {
			continue // a package internal/ gains, before it is listed
		}
		if to.Name == from.Name {
			continue
		}
		if !from.Allows(imp) {
			bad = append(bad, imp)
		}
	}
	slices.Sort(bad)
	return bad
}
