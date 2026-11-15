package window

import (
	ui "github.com/egoist/mygo/ui"

	"hyla/internal/proto"
	"hyla/internal/ui/k8sdialog"
	"hyla/internal/ui/theme"
)

// The app, as the pods of the K8s project ask it.
var _ k8sdialog.App = (*App)(nil)

// k8sPod is a pod the sidebar's K8s project lists.
type k8sPod = k8sdialog.Pod

func (a *App) PrefsK8sProject() string { return a.Prefs.K8sProject }

func (a *App) SetPrefsK8sProject(id string) { a.Prefs.K8sProject = id }

func (a *App) PrefsK8sPods() []k8sdialog.Pod { return a.Prefs.K8sPods }

func (a *App) SetPrefsK8sPods(ps []k8sdialog.Pod) { a.Prefs.K8sPods = ps }

func (a *App) AddPendingK8s(s proto.Session) { a.addPending(a.Machines[0], s) }

// AskK8sPod opens the dialog.
func (a *App) AskK8sPod() {
	if a.K8sAsk == nil {
		a.K8sAsk = &k8sdialog.Ask{Open: true}
	}
}

// k8sView draws the dialog while it is open.
func (a *App) k8sView(c *ui.Context, pal *theme.Palette) { k8sdialog.Show(a, c, pal) }

// k8sPodOfSession is the pod a session is the terminal of, nil for none.
func (a *App) k8sPodOfSession(id string) *k8sPod { return k8sdialog.OfSession(a, id) }

// ClearK8sProject forgets the K8s project.
func (a *App) ClearK8sProject() { k8sdialog.Clear(a) }

// k8sCommand reads a kubectl exec command.
func k8sCommand(cmd string) ([]string, string, error) { return k8sdialog.Command(cmd) }
