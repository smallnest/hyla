package search

import (
	"slices"
	"testing"
)

func TestResultsWithPlugins(t *testing.T) {
	pages := []Page{
		{ID: "general", Title: "General"},
		{ID: "plugins", Title: "Plugins"},
		{ID: "plugin:com.example.radar:main", Title: "Git Radar"},
	}
	plugins := []PluginEntry{
		{
			Page:     "plugins",
			Label:    "Gateway URL",
			Words:    "url Custom gateway endpoint Magpie hyla.magpie",
			PluginID: "hyla.magpie",
		},
		{
			Page:     "plugins",
			Label:    "API Token",
			Words:    "token Secret credential Demo com.example.demo",
			PluginID: "com.example.demo",
		},
		{
			Page:     "plugin:com.example.radar:main",
			Label:    "Git Radar",
			Words:    "com.example.radar main Radar workspace scanner",
			PluginID: "",
		},
		{
			Page:     "general",
			Label:    "Auto Sync",
			Words:    "sync interval plugin",
			PluginID: "",
		},
	}

	if got, pids := ResultsWithPlugins("   ", nil, pages, plugins, func(id string) string { return id }); got != nil || pids != nil {
		t.Fatalf("empty query should return nil, nil; got %v, %v", got, pids)
	}

	// Match by setting key + description inside Settings › Plugins.
	got, pids := ResultsWithPlugins("gateway endpoint", nil, pages, plugins, func(id string) string { return id })
	if !slices.Equal(got["plugins"], []string{"Gateway URL"}) {
		t.Fatalf("expected Gateway URL match on plugins, got %v", got)
	}
	if !slices.Equal(pids["plugins"], []string{"hyla.magpie"}) {
		t.Fatalf("expected pluginID hyla.magpie for plugins match, got %v", pids)
	}

	// Match standalone settings.page by title and by plugin words.
	gotPage, pidsPage := ResultsWithPlugins("workspace scanner", nil, pages, plugins, func(id string) string { return id })
	if !slices.Equal(gotPage["plugin:com.example.radar:main"], []string{"Git Radar"}) {
		t.Fatalf("expected Git Radar match on standalone page, got %v", gotPage)
	}
	if len(pidsPage["plugins"]) != 0 {
		t.Fatalf("expected no plugins drawer match, got %v", pidsPage)
	}

	// Match built-in Settings page slot entry.
	gotGen, _ := ResultsWithPlugins("auto sync", nil, pages, plugins, func(id string) string { return id })
	if !slices.Contains(gotGen["general"], "Auto Sync") {
		t.Fatalf("expected Auto Sync on general page, got %v", gotGen)
	}

	// Existing Results remains backward compatible without extras.
	if base := Results("font-size", nil, pages, func(id string) string { return id }); !slices.Equal(base["terminal"], []string{"Size"}) {
		t.Fatalf("expected backward-compatible font-size match in Results, got %v", base)
	}
	if empty := Results("zzzz-no-match", nil, pages, func(id string) string { return id }); len(empty) != 0 {
		t.Fatalf("expected empty map for unmatched query, got %v", empty)
	}
}
