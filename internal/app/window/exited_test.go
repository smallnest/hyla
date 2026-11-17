package window

import (
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"hyla/internal/proto"
	"hyla/locales"
)

// A pane whose shell exited says so over the last of its screen, and gives
// the way to start it again: a dead terminal takes no input.
func TestExitedPaneBar(t *testing.T) {
	a := testApp(t)
	s := a.session("s1")
	p := &s.Panes[0]
	p.Status, p.ExitCode = proto.Exited, 130
	o := a.TermOptions(p.ID, nil)
	o.Conn = &pipe{}
	trm, err := terminal.New(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { trm.Close() })
	a.Terms[p.ID] = &termEntry{term: trm, started: time.Now()}

	tt := ui.NewTester(a.view, 1100, 700)
	for i := 0; i < 20 && tt.FramePending(); i++ {
		tt.Frame()
		tt.Advance(time.Second)
	}
	tt.Frame()
	shot(t, tt, "pane-exited")
	for _, want := range []string{locales.PaneExited("130"), locales.PaneExitedDetail(), locales.CommonRestart(), locales.PaneClose()} {
		if _, ok := tt.Find(want); !ok {
			t.Errorf("no %q on the exited pane", want)
		}
	}
}
