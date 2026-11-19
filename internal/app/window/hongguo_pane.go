package window

import (
	"errors"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/video"

	"hyla/internal/app/hongguo"
	"hyla/internal/proto"
	"hyla/internal/ui/hongguoview"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

var _ hongguoview.App = (*App)(nil)

// openHongguo opens the Hongguo short drama pane beside the focused pane, or focuses
// an already-open Hongguo pane in the selected session.
func (a *App) openHongguo() {
	s := a.session(a.Selected)
	if s == nil {
		return
	}
	if p := hongguoPaneOf(s); p != nil {
		a.setFocus(s, p.ID)
		return
	}
	at := a.focusOf(s)
	if at == "" {
		return
	}
	a.split(s, proto.CreateParams{Split: at, Direction: "right", Hongguo: true})
}

func hongguoPaneOf(s *proto.Session) *proto.Pane {
	for i := range s.Panes {
		if s.Panes[i].HongguoPane() {
			return &s.Panes[i]
		}
	}
	return nil
}

// HongguoService returns the lazy-initialized Hongguo backend service.
func (a *App) HongguoService() *hongguo.Service {
	if a.HongguoSvc == nil {
		a.HongguoSvc = hongguo.NewService("")
	}
	return a.HongguoSvc
}

// HongguoState returns or creates persistent UI state for the given Hongguo pane ID.
func (a *App) HongguoState(paneID string) *hongguoview.State {
	if a.Hongguo == nil {
		a.Hongguo = map[string]*hongguoview.State{}
	}
	st := a.Hongguo[paneID]
	if st == nil {
		st = hongguoview.NewState(paneID, a.HongguoService())
		a.Hongguo[paneID] = st
	}
	return st
}

// RunMain executes fn on the app main thread (overridable in headless tests).
func (a *App) RunMain(fn func()) {
	if a.hongguoMain != nil {
		a.hongguoMain(fn)
		return
	}
	a.Main(fn)
}

// OpenVideo opens a decrypted local file or remote stream URL via MyGo's native video engine.
func (a *App) OpenVideo(localPath, streamURL string) (*ui.VideoPlayer, error) {
	if a.hongguoOpenVideo != nil {
		return a.hongguoOpenVideo(localPath, streamURL)
	}
	if localPath != "" {
		return video.Open(localPath)
	}
	if streamURL != "" {
		return video.OpenURL(streamURL)
	}
	return nil, errors.New("hongguo: missing video source")
}

// HongguoBody draws the Hongguo short drama pane.
func (a *App) HongguoBody(c *ui.Context, pal *theme.Palette, s *proto.Session, p *proto.Pane) {
	hongguoview.Pane(a, c, pal, s, p)
}

// DropHongguo saves watch progress and releases any video player owned by pane.
func (a *App) DropHongguo(pane string) {
	st := a.Hongguo[pane]
	if st == nil {
		return
	}
	st.Close(a.HongguoService())
	delete(a.Hongguo, pane)
}

func (a *App) dropAllHongguo() {
	for id := range a.Hongguo {
		a.DropHongguo(id)
	}
}

// checkHongguoAutoPause pauses playing Hongguo panes with AutoPause enabled when a coding
// agent transitions from Working to non-Working.
func (a *App) checkHongguoAutoPause(first bool) {
	if a.hongguoWorking == nil {
		a.hongguoWorking = map[string]bool{}
	}

	nextWorking := map[string]bool{}
	anyWorking := false
	hasTerminal := map[string]bool{}
	for i := range a.St.Sessions {
		s := &a.St.Sessions[i]
		working := false
		for j := range s.Panes {
			pn := &s.Panes[j]
			if pn.Shown() {
				continue
			}
			hasTerminal[s.ID] = true
			if pn.Status == proto.Working {
				working = true
			}
		}
		nextWorking[s.ID] = working
		if working {
			anyWorking = true
		}
	}
	nextWorking["*"] = anyWorking

	if !first {
		for i := range a.St.Sessions {
			s := &a.St.Sessions[i]
			key := s.ID
			if !hasTerminal[s.ID] {
				key = "*"
			}
			wasWorking := a.hongguoWorking[key]
			nowWorking := nextWorking[key]
			if !wasWorking || nowWorking {
				continue
			}
			for j := range s.Panes {
				pn := &s.Panes[j]
				if !pn.HongguoPane() {
					continue
				}
				if st := a.Hongguo[pn.ID]; st != nil {
					st.PauseForAgent(a.HongguoService(), locales.HongguoAutoPausedNotice())
				}
			}
		}
	}

	a.hongguoWorking = nextWorking
}
