package uihost

import (
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/plugins/rpc"
	"hyla/internal/ui/theme"
)

func TestRenderSelectToggleSpinnerAndPropSync(t *testing.T) {
	rs := NewRenderState()
	var (
		mu     sync.Mutex
		events []struct {
			id   string
			typ  string
			val  any
		}
		inputVal = "initial"
	)

	buildTree := func() *rpc.UINode {
		return &rpc.UINode{
			Type: "column",
			Key:  "root",
			Children: []*rpc.UINode{
				{
					ID:    "t1",
					Type:  "text",
					Key:   "txt-val",
					Props: map[string]any{"value": "Value Fallback"},
				},
				{
					ID:    "sp1",
					Type:  "spinner",
					Key:   "spin",
					Props: map[string]any{"label": "Working"},
				},
				{
					ID:      "qbtn",
					Type:    "button",
					Key:     "qbtn",
					Props:   map[string]any{"label": "Quiet", "variant": "quiet"},
					OnClick: "quietClick",
				},
				{
					ID:       "tog",
					Type:     "toggle",
					Key:      "tog",
					Props:    map[string]any{"value": false},
					OnChange: "togChanged",
				},
				{
					ID:   "sel",
					Type: "select",
					Key:  "sel",
					Props: map[string]any{
						"value": "one",
						"options": []any{
							map[string]any{"value": "one", "label": "One"},
							map[string]any{"value": "two", "label": "Two"},
						},
					},
					OnChange: "selChanged",
				},
				{
					ID:    "inp",
					Type:  "textInput",
					Key:   "inp",
					Props: map[string]any{"value": inputVal, "placeholder": "Type"},
				},
			},
		}
	}

	tt := ui.NewTester(func(c *ui.Context) {
		pal := &theme.Light
		Render(c, RenderContext{
			PluginID: "com.example.render",
			SlotID:   "card",
			Palette:  pal,
			State:    rs,
			OnEvent: func(componentID, eventType string, value any) {
				mu.Lock()
				events = append(events, struct {
					id  string
					typ string
					val any
				}{componentID, eventType, value})
				mu.Unlock()
			},
		}, buildTree())
	}, 600, 600)
	tt.Frame()

	// Verify text "value" fallback & spinner "label" rendered.
	if !tt.HasText("Value Fallback") {
		t.Fatalf("expected text value fallback to render, texts=%v", tt.Texts())
	}
	if !tt.HasText("Working") {
		t.Fatalf("expected spinner label to render, texts=%v", tt.Texts())
	}

	// Click quiet button.
	if err := tt.Click("Quiet"); err != nil {
		t.Fatalf("click quiet button: %v", err)
	}

	// Click toggle switch.
	var sw *ui.AccessInfo
	var popup *ui.AccessInfo
	for _, a := range tt.AccessAll() {
		if a.Role == ui.RoleSwitch {
			cp := a
			sw = &cp
		}
		if a.Name == "One" || a.Value == "One" {
			cp := a
			popup = &cp
		}
	}
	if sw == nil {
		t.Fatalf("missing toggle RoleSwitch in %+v", tt.AccessAll())
	}
	tt.ClickAt(sw.Bounds.X+sw.Bounds.W/2, sw.Bounds.Y+sw.Bounds.H/2)

	// Open select popup and choose "Two".
	if popup == nil {
		t.Fatalf("missing select trigger 'One' in %+v", tt.AccessAll())
	}
	tt.ClickAt(popup.Bounds.X+popup.Bounds.W/2, popup.Bounds.Y+popup.Bounds.H/2)
	tt.Frame()
	if err := tt.Click("Two"); err != nil {
		t.Fatalf("click select option Two: %v", err)
	}

	// Wait briefly for async event goroutines.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(events)
		mu.Unlock()
		if n >= 3 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	got := append([]struct {
		id  string
		typ string
		val any
	}(nil), events...)
	mu.Unlock()

	var sawQuiet, sawToggle, sawSelect bool
	for _, ev := range got {
		switch {
		case ev.id == "qbtn" && ev.typ == "quietClick":
			sawQuiet = true
		case ev.id == "tog" && ev.typ == "togChanged" && ev.val == true:
			sawToggle = true
		case ev.id == "sel" && ev.typ == "selChanged" && ev.val == "two":
			sawSelect = true
		}
	}
	if !sawQuiet || !sawToggle || !sawSelect {
		t.Fatalf("missing interactive events (quiet=%v toggle=%v select=%v), events=%+v", sawQuiet, sawToggle, sawSelect, got)
	}

	// Verify prop sync on textInput when plugin changes Props["value"].
	inputVal = "updated-by-plugin"
	tt.Frame()
	if !tt.HasText("updated-by-plugin") {
		var tfVal string
		for _, a := range tt.AccessAll() {
			if a.Role == ui.RoleTextField {
				tfVal = a.Value
			}
		}
		if tfVal != "updated-by-plugin" {
			t.Fatalf("expected textInput prop update to sync to 'updated-by-plugin', got %q", tfVal)
		}
	}
}
