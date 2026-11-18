package pets

import (
	"slices"
	"testing"

	pluginmanifest "hyla/internal/plugins/manifest"
)

func TestReloadPluginPets(t *testing.T) {
	t.Cleanup(func() { ReloadPlugins(nil) })

	ReloadPlugins([]pluginmanifest.PetAsset{{
		PluginID: "com.example.pets",
		ID:       "pixel-frog",
		Dir:      "/tmp/nonexistent-pet-dir",
	}})
	if !slices.Contains(Installed(), "pixel-frog") {
		t.Fatal("Installed() should include plugin-contributed pet id")
	}
}
