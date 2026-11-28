package actions

import (
	"sync"
	"testing"
)

func TestPluginActionsLifecycle(t *testing.T) {
	t.Cleanup(ClearPluginActions)

	baseCount := len(All)
	if len(List()) != baseCount {
		t.Fatalf("List() len = %d, want %d", len(List()), baseCount)
	}

	SetPluginActions("com.example.zeta", []Action{{
		ID:    "com.example.zeta.run",
		Title: "Zeta Run",
		Menu:  "Plugins",
		Keys:  "CmdOrCtrl+Alt+Z",
	}})
	SetPluginActions("com.example.alpha", []Action{{
		ID:     "com.example.alpha.open",
		Title:  "Alpha Open",
		Remote: true,
	}})

	list := List()
	if len(list) != baseCount+2 {
		t.Fatalf("List() len = %d, want %d", len(list), baseCount+2)
	}
	if list[baseCount].ID != "com.example.alpha.open" || list[baseCount+1].ID != "com.example.zeta.run" {
		t.Fatalf("unexpected sorted order: %q, %q", list[baseCount].ID, list[baseCount+1].ID)
	}
	if a, ok := Find("com.example.alpha.open"); !ok || a.PluginID != "com.example.alpha" || !a.Remote {
		t.Fatalf("Find plugin action failed: %+v, ok=%v", a, ok)
	}
	if a, ok := Find("session.new"); !ok || a.PluginID != "" {
		t.Fatalf("Find builtin failed: %+v, ok=%v", a, ok)
	}

	RemovePluginActions("com.example.alpha")
	if _, ok := Find("com.example.alpha.open"); ok {
		t.Fatal("removed plugin action still found")
	}
}

func TestIsPluginActionID(t *testing.T) {
	t.Cleanup(ClearPluginActions)

	SetDisabledPluginActions([]string{"org.disabled.tool.run"})
	cases := []struct {
		id   string
		want bool
	}{
		{"", false},
		{"settings.open", false},
		{"pane.split.bogus", false},
		{"foo.bar", false},
		{"plugin.custom", true},
		{"org.disabled.tool.run", true},
		{"com.acme.notes.open", true},
		{"io.example.tool.sub.cmd", true},
		{"com.acme_bad.notes.open", false},
	}
	for _, tc := range cases {
		if got := IsPluginActionID(tc.id); got != tc.want {
			t.Errorf("IsPluginActionID(%q) = %v, want %v", tc.id, got, tc.want)
		}
	}
}

func TestPluginActionsConcurrent(t *testing.T) {
	t.Cleanup(ClearPluginActions)
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for range 50 {
				SetPluginActions("com.example.worker", []Action{{ID: "com.example.worker.ping"}})
				_ = List()
				_, _ = Find("com.example.worker.ping")
				_ = IsPluginActionID("com.example.worker.ping")
				if i%2 == 0 {
					RemovePluginActions("com.example.worker")
				}
			}
		}(i)
	}
	wg.Wait()
}

func TestDefaultShortcutsUniqueAndNoDockConflict(t *testing.T) {
	seenMac := make(map[string]string)
	seenPC := make(map[string]string)
	for _, a := range All {
		if a.ID == "pane.fork" && a.Keys == "CmdOrCtrl+Alt+D" {
			t.Fatalf("pane.fork defaults to CmdOrCtrl+Alt+D, which conflicts with macOS Dock shortcut")
		}
		if a.Keys != "" {
			if prev, dup := seenMac[a.Keys]; dup {
				t.Errorf("duplicate macOS shortcut %q on %q and %q", a.Keys, prev, a.ID)
			}
			seenMac[a.Keys] = a.ID
		}
		pcKey := a.Keys
		if override, ok := pcKeys[a.ID]; ok {
			pcKey = override
		}
		if pcKey != "" {
			if prev, dup := seenPC[pcKey]; dup {
				t.Errorf("duplicate PC shortcut %q on %q and %q", pcKey, prev, a.ID)
			}
			seenPC[pcKey] = a.ID
		}
	}
}
