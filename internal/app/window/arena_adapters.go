package window

import "hyla/internal/ui/arenadialog"

var _ arenadialog.App = (*App)(nil)

func (a *App) ArenaAskNow() *arenadialog.Ask { return a.ArenaAsk }

func (a *App) SetArenaAsk(d *arenadialog.Ask) { a.ArenaAsk = d }
