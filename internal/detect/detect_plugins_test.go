package detect

import (
	"sync"
	"testing"
)

func TestReloadPluginsAndConcurrency(t *testing.T) {
	t.Cleanup(ClearPlugins)

	if Known("aider") {
		t.Fatal("aider should not be known initially")
	}

	err := ReloadPlugins([]PluginManifest{{
		Agent: "aider",
		Name:  "aider.toml",
		TOML: []byte(`
[[rules]]
id = "aider.waiting"
state = "blocked"
priority = 100
region = "whole_recent"
contains = ["allow edits"]

[[rules]]
id = "aider.working"
state = "working"
priority = 50
region = "whole_recent"
regex = ["Thinking\\.\\.\\."]
`),
	}})
	if err != nil {
		t.Fatalf("ReloadPlugins: %v", err)
	}
	if !Known("aider") {
		t.Fatal("aider should be known after ReloadPlugins")
	}

	res := Detect("aider", Input{Screen: "Should I allow edits to main.go?"})
	if res.State != Blocked || res.Rule != "aider.waiting" {
		t.Fatalf("got %+v, want blocked aider.waiting", res)
	}
	res = Detect("aider", Input{Screen: "Thinking..."})
	if res.State != Working || res.Rule != "aider.working" {
		t.Fatalf("got %+v, want working aider.working", res)
	}

	// Bad TOML / bad regex returns error without breaking valid manifest loading.
	err = ReloadPlugins([]PluginManifest{
		{
			Agent: "bad",
			TOML:  []byte(`[[rules]]\nid="bad"\nstate="working"\nregex=["["]\n`),
		},
		{
			Agent: "aider",
			TOML:  []byte("[[rules]]\nid = \"aider.idle\"\nstate = \"idle\"\ncontains = [\"> \"]\n"),
		},
	})
	if err == nil {
		t.Fatal("expected error for malformed plugin TOML")
	}

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 50 {
				if i%10 == 0 {
					_ = ReloadPlugins([]PluginManifest{{
						Agent: "aider",
						TOML:  []byte("[[rules]]\nid = \"aider.work\"\nstate = \"working\"\ncontains = [\"busy\"]\n"),
					}})
				}
				_ = Detect("aider", Input{Screen: "busy"})
				_ = Detect("claude", Input{Screen: "Working"})
			}
		}()
	}
	wg.Wait()
}
