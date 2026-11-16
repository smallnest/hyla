package marks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/egoist/mygo/ui"

	"hyla/locales"
)

// Button draws a card's button, and tells whether the user clicked it.
type ButtonFn func(c *ui.Context, key, label string, primary bool) bool

// Hooks are what the logos card needs of the app that draws it: a button
// of the panel's look, and what to do with the file the user chose.
var Hooks struct {
	Button ButtonFn
	// Give keeps the file the user chose for an agent's logo, and a nil
	// one takes back the user's logo of an agent whose own is one Hyla
	// reads.
	Give func(agent, path string)
}

// wired reports whether Hooks are set, as the app sets them.
func wired() bool { return Hooks.Button != nil && Hooks.Give != nil }

// CardDrawer is the Info view's card, which the logos card draws in: the
// panel's own look, which marks does not know.
type CardDrawer func(c *ui.Context, fold string, title, aside string, body func())

// Logos draws the Info view's Agents card, through card: the agent of the
// pane the user is in, with the logo it wears, and the user's logo of an
// agent Hyla has none for, which they give it here: give is called with
// the file they chose, which the app keeps, and the logo reads again.
func Logos(c *ui.Context, agent string, card CardDrawer) {
	if !wired() {
		return
	}
	card(c, "agents", locales.InfoAgents(), "", func() {
		ui.Column(c).Padding(2, 10, 10, 10).Gap(8).Children(func() {
			if agent == "" {
				ui.Text(c, locales.InfoAgentsNone()).FontSize(12.5)
				return
			}
			ui.Text(c, locales.InfoAgentsDetail()).FontSize(12).Wrap()
			LogosRow(c, agent)
			if hasUserMark(agent) {
				ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
					if Hooks.Button(c, "logo-remove", locales.InfoAgentsRemove(), false) {
						Hooks.Give(agent, "")
					}
				})
			}
		})
	})
}

// LogosRow draws an agent's logo and name, with the button that takes the
// user's logo of it from a file.
func LogosRow(c *ui.Context, agent string) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if src, ok := LogoOf(agent); ok {
			if bitmap, isBitmap := src.(*ui.Bitmap); isBitmap {
				ui.Image(c, bitmap).Size(18, 18).Radius(4).Shrink(0)
			} else if full, ok := MarkFull(agent); ok && full != nil {
				ui.Image(c, full).Size(18, 18).Shrink(0)
			} else if logo, _, ok := Agent(agent); ok {
				ui.Icon(c, logo).Size(18, 18).Shrink(0)
			}
		}
		ui.Text(c, agent).FontSize(13).SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
		if Hooks.Button(c, "logo-add", locales.InfoAgentsAdd(), true) {
			path, err := PickImage()
			if err != nil || path == "" {
				return
			}
			Hooks.Give(agent, path)
		}
	})
}

// hasUserMark tells whether the user gave an agent a logo.
func hasUserMark(agent string) bool {
	agent = strings.ToLower(strings.TrimSpace(agent))
	entries, err := os.ReadDir(MarksDir())
	if err != nil {
		return false
	}
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		if strings.TrimSuffix(name, filepath.Ext(name)) == agent {
			return true
		}
	}
	return false
}

// PickImage asks for an image file, with the dialog of the machine's
// windows: the path it says, or "" when the user cancels.
func PickImage() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("osascript", "-e",
			`POSIX path of (choose file with prompt "Choose a logo" of type {"public.image"})`).Output()
		if err != nil {
			return "", nil // the user cancelled
		}
		return strings.TrimSpace(string(out)), nil
	case "windows":
		script := `Add-Type -AssemblyName System.Windows.Forms;` +
			`$d = New-Object System.Windows.Forms.OpenFileDialog;` +
			`$d.Filter = 'Images|*.png;*.jpg;*.jpeg;*.gif;*.webp';` +
			`if ($d.ShowDialog() -eq 'OK') { $d.FileName }`
		out, err := exec.Command("powershell", "-NoProfile", "-Command", script).Output()
		if err != nil {
			return "", nil
		}
		return strings.TrimSpace(string(out)), nil
	default:
		for _, cmd := range []string{"zenity", "kdialog"} {
			if _, err := exec.LookPath(cmd); err != nil {
				continue
			}
			var args []string
			if cmd == "zenity" {
				args = []string{"--file-selection", "--title=Choose a logo", "--file-filter=Images | *.png *.jpg *.jpeg *.gif *.webp"}
			} else {
				args = []string{"--getopenfilename", ".", "Images (*.png *.jpg *.jpeg *.gif *.webp)"}
			}
			out, err := exec.Command(cmd, args...).Output()
			if err != nil {
				return "", nil
			}
			return strings.TrimSpace(string(out)), nil
		}
		return "", fmt.Errorf("no file dialog")
	}
}
