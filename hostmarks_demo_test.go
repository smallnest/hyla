package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/proto"
	"hyla/locales"
)

// TestSidebarHostMarks lays the sidebar out with the SSH and K8s projects
// among folders, for a look at their marks in $HYLA_SHOTS.
func TestSidebarHostMarks(t *testing.T) {
	a := testApp(t)
	a.st.Projects = []proto.Project{
		{ID: "p1", Name: "hyla", Dir: "/Users/smallnest/ai/hyla"},
		{ID: "p-ssh", Name: locales.SshHostsProject()},
		{ID: "p-k8s", Name: locales.K8sPodsProject()},
	}
	a.prefs.SSHProject, a.prefs.K8sProject = "p-ssh", "p-k8s"
	a.prefs.SSHHosts = []sshHost{{Name: "example.com", Addr: "root@example.com:2222"}, {Name: "devbox", Addr: "me@devbox"}}
	a.prefs.K8sPods = []k8sPod{{Name: "api-7d9f", Command: "kubectl exec -it api-7d9f -n prod -- bash"}}
	a.loaded = true
	for _, dark := range []bool{false, true} {
		name := "sidebar-host-marks"
		if dark {
			name = "sidebar-host-marks-dark"
		}
		tt := ui.NewTesterWith(a.view, 420, 480, ui.TesterOptions{Dark: dark})
		for i := 0; i < 60 && tt.FramePending(); i++ {
			tt.Frame()
			tt.Advance(time.Second)
		}
		shot(t, tt, name)
	}
}
