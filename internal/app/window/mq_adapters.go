package window

import "hyla/internal/ui/mqdialog"

var _ mqdialog.App = (*App)(nil)

func (a *App) MQAskNow() *mqdialog.Ask { return a.MQAsk }

func (a *App) SetMQAsk(d *mqdialog.Ask) { a.MQAsk = d }
