package daemon

import (
	"testing"

	"hyla/internal/proto"
)

// A Hongguo pane runs no shell: the daemon makes it as a pane that shows
// Hongguo short dramas, and a restore brings it back as one.
func TestHongguoPane(t *testing.T) {
	d := testDaemon(t)
	pn, err := d.startPane("s1", proto.CreateParams{Hongguo: true}, "/tmp")
	if err != nil {
		t.Fatal(err)
	}
	if !pn.info.HongguoPane() || !pn.info.Shown() || pn.term != nil || pn.proc != nil {
		t.Fatalf("pane %+v, term %v", pn.info, pn.term != nil)
	}
	if pn.info.URL != "" || pn.info.File != "" || pn.info.Timeline != "" || pn.info.Subagents {
		t.Errorf("a hongguo pane shows something else: %+v", pn.info)
	}
	r := d.restorePane("s1", savedPane{ID: "p2", Dir: "/tmp", Hongguo: true})
	if r == nil || !r.info.HongguoPane() || !r.info.Shown() || r.term != nil {
		t.Fatalf("restored %+v", r.info)
	}
}
