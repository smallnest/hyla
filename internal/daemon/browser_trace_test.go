package daemon

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"hyla/internal/proto"
)

func makeTestPNG(t *testing.T, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestBrowserRecordAndReplay(t *testing.T) {
	d := testDaemon(t)
	ses, err := d.create(proto.CreateParams{URL: "http://localhost:3000"})
	if err != nil {
		t.Fatal(err)
	}
	paneID := ses.Panes[0].ID

	// Register a fake subscriber that answers browser requests.
	var shotColor atomic.Int32 // 0 = blue, 1 = red
	blue := makeTestPNG(t, color.RGBA{0, 0, 255, 255})
	red := makeTestPNG(t, color.RGBA{255, 0, 0, 255})

	sub := &controlConn{done: make(chan struct{}), out: make(chan proto.Message, 16)}
	d.mu.Lock()
	d.subs[sub] = struct{}{}
	d.mu.Unlock()
	t.Cleanup(func() { close(sub.done) })

	go func() {
		for msg := range sub.out {
			if msg.Event != "browser" || msg.Browser == nil {
				continue
			}
			var ans proto.BrowserAnswer
			ans.Req = msg.Browser.Req
			switch msg.Browser.Op {
			case "screenshot":
				if shotColor.Load() == 0 {
					ans.Result = &proto.BrowserResult{PNG: blue}
				} else {
					ans.Result = &proto.BrowserResult{PNG: red}
				}
			default:
				ans.Result = &proto.BrowserResult{Text: "ok"}
			}
			raw, _ := json.Marshal(ans)
			_, _ = d.call(sub, "browser.answer", raw)
		}
	}()

	flowPath := filepath.Join(t.TempDir(), "flows", "login.json")
	if _, err := d.browserRecordStart(proto.BrowserRecordParams{Pane: paneID, Name: "login", Out: flowPath}); err != nil {
		t.Fatalf("browserRecordStart failed: %v", err)
	}

	// Execute steps via RPC.
	for _, bp := range []proto.BrowserParams{
		{ID: paneID, Op: "open", Text: "http://localhost:3000/login"},
		{ID: paneID, Op: "snapshot"}, // read-only, should NOT be recorded
		{ID: paneID, Op: "click", Selector: "#submit"},
		{ID: paneID, Op: "screenshot"},
	} {
		raw, _ := json.Marshal(bp)
		if _, err := d.call(sub, "browser", raw); err != nil {
			t.Fatalf("browser %s failed: %v", bp.Op, err)
		}
	}

	st, err := d.browserRecordStop(proto.BrowserRecordParams{Pane: paneID})
	if err != nil {
		t.Fatalf("browserRecordStop failed: %v", err)
	}
	if st.Steps != 3 || st.SavedPath != flowPath {
		t.Fatalf("unexpected record stop status: %+v", st)
	}

	pn := d.findPane(paneID)
	// First replay writes baseline.
	rep1, err := d.browserReplay(sub, pn, proto.BrowserReplayParams{
		Pane:           paneID,
		FlowPath:       flowPath,
		UpdateBaseline: true,
	})
	if err != nil || !rep1.Passed || len(rep1.Steps) != 3 {
		t.Fatalf("replay1 = (%+v, %v)", rep1, err)
	}
	if _, err := os.Stat(rep1.Screenshot); err != nil {
		t.Errorf("missing screenshot %s: %v", rep1.Screenshot, err)
	}

	// Second replay with identical pixels -> 0 diff.
	rep2, err := d.browserReplay(sub, pn, proto.BrowserReplayParams{
		Pane:     paneID,
		FlowPath: flowPath,
	})
	if err != nil || !rep2.Passed || rep2.BaselineDiffRatio != 0 {
		t.Fatalf("replay2 = (%+v, %v), want diff=0", rep2, err)
	}

	// Third replay with changed screenshot -> diff == 1.0.
	shotColor.Store(1)
	rep3, err := d.browserReplay(sub, pn, proto.BrowserReplayParams{
		Pane:     paneID,
		FlowPath: flowPath,
	})
	if err != nil || !rep3.Passed || rep3.BaselineDiffRatio != 1.0 {
		t.Fatalf("replay3 = (%+v, %v), want diff=1.0", rep3, err)
	}
}
