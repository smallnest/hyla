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

func TestRenderToggleOptimisticSegmentedDisabledAndColors(t *testing.T) {
	if c, ok := ResolveColor("$bg", &theme.Light); !ok || c != theme.Light.Card {
		t.Fatalf("ResolveColor($bg) = (%+v, %v), want Card", c, ok)
	}
	if c, ok := ResolveColor("rgb(12, 34, 56)", &theme.Light); !ok || c.R != 12 || c.G != 34 || c.B != 56 {
		t.Fatalf("ResolveColor(rgb) = (%+v, %v)", c, ok)
	}

	rs := NewRenderState()
	var (
		mu     sync.Mutex
		events []string
	)
	toggleProp := false
	segProp := "a"

	tt := ui.NewTester(func(c *ui.Context) {
		tree := &rpc.UINode{
			Type: "column",
			Key:  "root2",
			Children: []*rpc.UINode{
				{
					ID:       "opt-tog",
					Type:     "toggle",
					Key:      "opt-tog",
					Props:    map[string]any{"value": toggleProp},
					OnChange: "togEv",
				},
				{
					ID:   "seg1",
					Type: "segmented",
					Key:  "seg1",
					Props: map[string]any{
						"value": segProp,
						"options": []any{
							map[string]any{"value": "a", "label": "Alpha"},
							map[string]any{"value": "b", "label": "Beta"},
						},
					},
					OnChange: "segEv",
				},
				{
					ID:      "dis-btn",
					Type:    "button",
					Key:     "dis-btn",
					Props:   map[string]any{"label": "CannotClick", "disabled": true},
					OnClick: "disClick",
				},
				{
					ID:    "wrap-txt",
					Type:  "text",
					Key:   "wrap-txt",
					Props: map[string]any{"text": "Wrapped line", "wrap": true},
				},
			},
		}
		Render(c, RenderContext{
			PluginID: "com.example.render2",
			SlotID:   "card",
			Palette:  &theme.Light,
			State:    rs,
			OnEvent: func(componentID, eventType string, value any) {
				mu.Lock()
				events = append(events, componentID+":"+eventType)
				mu.Unlock()
			},
		}, tree)
	}, 600, 500)
	tt.Frame()

	// Click toggle: optimistic true must survive next frame while toggleProp is still false.
	var sw *ui.AccessInfo
	for _, a := range tt.AccessAll() {
		if a.Role == ui.RoleSwitch {
			cp := a
			sw = &cp
			break
		}
	}
	if sw == nil {
		t.Fatal("missing RoleSwitch")
	}
	tt.ClickAt(sw.Bounds.X+sw.Bounds.W/2, sw.Bounds.Y+sw.Bounds.H/2)
	tt.Frame()
	if gotPtr := rs.syncBoolProp("plg:com.example.render2:card:root2:opt-tog", toggleProp); !*gotPtr {
		t.Fatal("expected optimistic toggle state preserved when prop remained unchanged")
	}

	// Click Beta in segmented control and disabled button.
	var foundDisBtn bool
	for _, a := range tt.AccessAll() {
		if a.Name == "CannotClick" {
			foundDisBtn = true
			if a.Role != ui.RoleButton || !a.Disabled {
				t.Fatalf("expected disabled button to preserve RoleButton+Disabled, got role=%v disabled=%v", a.Role, a.Disabled)
			}
		}
	}
	if !foundDisBtn {
		t.Fatal("disabled button missing from accessibility tree")
	}
	if err := tt.Click("Beta"); err != nil {
		t.Fatalf("click Beta: %v", err)
	}
	_ = tt.Click("CannotClick")
	tt.Frame()

	if segPtr := rs.syncStringProp("plg:com.example.render2:card:root2:seg1", segProp); *segPtr != "b" {
		t.Fatalf("expected persistent segmented state = 'b', got %q", *segPtr)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(events)
		mu.Unlock()
		if n >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	var sawTog, sawSeg, sawDis bool
	for _, ev := range events {
		switch ev {
		case "opt-tog:togEv":
			sawTog = true
		case "seg1:segEv":
			sawSeg = true
		case "dis-btn:disClick":
			sawDis = true
		}
	}
	if !sawTog || !sawSeg {
		t.Fatalf("expected toggle + segmented events, got %v", events)
	}
	if sawDis {
		t.Fatalf("disabled button fired click event: %v", events)
	}
}
