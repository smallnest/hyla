package window

import (
	ui "github.com/egoist/mygo/ui"

	"hyla/internal/proto"
	"hyla/internal/remote"
	"hyla/internal/ui/confirm"
	"hyla/internal/ui/k8sdialog"
	"hyla/internal/ui/sshdialog"
	"hyla/internal/ui/theme"
)

// The app, as the machines of the SSH project ask it.
var _ sshdialog.App = (*App)(nil)

// sshHost is a machine the sidebar's SSH project lists.
type sshHost = sshdialog.Host

func (a *App) PrefsSSHProject() string { return a.Prefs.SSHProject }

func (a *App) SetPrefsSSHProject(id string) { a.Prefs.SSHProject = id }

func (a *App) PrefsSSHHosts() []sshdialog.Host { return a.Prefs.SSHHosts }

func (a *App) SetPrefsSSHHosts(hs []sshdialog.Host) { a.Prefs.SSHHosts = hs }

func (a *App) ProjectNow(id string) *proto.Project { return a.project(id) }

func (a *App) OpenProject(id string) { delete(a.Prefs.Collapsed, id) }

func (a *App) AddProject(p proto.Project) { a.St.Projects = append(a.St.Projects, p) }

func (a *App) AddPendingSSH(s proto.Session) { a.addPending(a.Machines[0], s) }

func (a *App) CopyAddress(addr string) { a.CopyText = addr }

func (a *App) ConfirmNow(q *confirm.Ask) { a.Confirm = q }

func (a *App) Prompt() *sshdialog.Ask { return a.SshAsk }

func (a *App) SetPrompt(d *sshdialog.Ask) { a.SshAsk = d }

func (a *App) SidebarRows() string { return a.settingsIn.SidebarRows }

func (a *App) DragAt() string { return a.DragAtAddr }

func (a *App) DragAfter() bool { return a.DragAfterSide }

func (a *App) SetDragAt(addr string, after bool) { a.DragAtAddr, a.DragAfterSide = addr, after }

func (a *App) K8sPrompt() *k8sdialog.Ask { return a.K8sAsk }

func (a *App) SetK8sPrompt(d *k8sdialog.Ask) { a.K8sAsk = d }

// AskSSHHost opens the dialog.
func (a *App) AskSSHHost() {
	if a.SshAsk == nil {
		a.SshAsk = &sshdialog.Ask{Open: true}
	}
}

// sshView draws the dialog while it is open.
func (a *App) SSH(c *ui.Context, pal *theme.Palette) { sshdialog.Show(a, c, pal) }

// sshHostRow draws a machine in the SSH project.
func (a *App) sshHostRow(c *ui.Context, pal *theme.Palette, i int) { sshdialog.Row(a, c, pal, i) }

// sshHostOfSession is the machine a session is the terminal of, nil for
// none.
func (a *App) sshHostOfSession(id string) *sshHost { return sshdialog.OfSession(a, id) }

// moveSSHHost puts a machine where another is.
func (a *App) moveSSHHost(from, to string, after bool) { sshdialog.Move(a, from, to, after) }

// removeSSHHost removes a machine from the SSH project.
func (a *App) removeSSHHost(addr string) { sshdialog.Remove(a, addr) }

// ClearSSHProject forgets the SSH project.
func (a *App) ClearSSHProject() { sshdialog.Clear(a) }

// sshArgs is ssh's command line for an address.
func sshArgs(addr remote.Addr) []string { return sshdialog.Args(addr) }
