package window

import "hyla/internal/ui/checkpointdialog"

var _ checkpointdialog.App = (*App)(nil)

func (a *App) CheckpointAskNow() *checkpointdialog.Ask { return a.CheckpointAsk }

func (a *App) SetCheckpointAsk(d *checkpointdialog.Ask) { a.CheckpointAsk = d }
