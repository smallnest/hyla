package daemon

import (
	"testing"

	"hyla/internal/proto"
)

// TestMoveSession moves a session to another project of the machine: the
// session joins the project, after its sessions.
func TestMoveSession(t *testing.T) {
	d := testDaemon(t)
	d.projects = append(d.projects, proto.Project{ID: "other", Name: "other"})
	one, err := d.create(proto.CreateParams{Project: "pr", URL: "localhost:3000"})
	if err != nil {
		t.Fatal(err)
	}
	two, err := d.create(proto.CreateParams{Project: "other", URL: "localhost:3001"})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.move(proto.MoveParams{ID: one.ID, Project: "other", Index: -1}); err != nil {
		t.Fatal(err)
	}
	st := d.state()
	var got []string
	for _, s := range st.Sessions {
		if s.Project == "other" {
			got = append(got, s.ID)
		}
	}
	if len(got) != 2 || got[0] != two.ID || got[1] != one.ID {
		t.Errorf("other's sessions = %v, want %s then %s", got, two.ID, one.ID)
	}
	if err := d.move(proto.MoveParams{ID: one.ID, Project: "pr"}); err != nil {
		t.Fatal(err)
	}
	for _, s := range d.state().Sessions {
		if s.ID == one.ID && s.Project != "pr" {
			t.Errorf("%s is in %s, want pr", one.ID, s.Project)
		}
	}
	if err := d.move(proto.MoveParams{ID: "nonesuch", Project: "pr", Index: -1}); err == nil {
		t.Error("a session that does not exist moved")
	}
}
