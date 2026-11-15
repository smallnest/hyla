package main

import (
	"context"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/settings"
	hylasync "hyla/internal/sync"
	"hyla/internal/ui/style"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

// syncDelay is how long a change waits before it goes up: the controls save
// as they change, and a run of them is one push.
const syncDelay = 30 * time.Second

// syncWait bounds a sync: a server that does not answer in that time leaves
// the next one to try.
const syncWait = 90 * time.Second

// syncState is what the app knows of syncing this machine's configuration:
// the server it syncs to, as the Sync page last saved it, and what the last
// sync did.
type syncState struct {
	cfg     hylasync.Config
	saved   hylasync.Config // as the file has it, for what changed
	loaded  bool
	busy    bool      // a sync runs
	pulled  bool      // the launch pull is done
	applied bool      // the settings being applied are a pull's, not a change
	due     time.Time // a change waiting for its push
	at      time.Time // the last attempt
	action  string
	host    string
	err     string
}

// syncTick pulls at launch, and pushes a change once it waited, as the Sync
// page says.
func (a *App) syncTick() {
	st := &a.sync
	if !st.loaded {
		st.cfg, st.saved, st.loaded = hylasync.Load(), hylasync.Load(), true
	}
	if !hylasync.Configured(st.cfg) || st.busy {
		return
	}
	if !st.pulled && a.loaded {
		st.pulled = true
		a.syncRun("")
		return
	}
	if st.due.IsZero() || time.Now().Before(st.due) {
		return
	}
	st.due = time.Time{}
	a.syncRun("")
}

// syncSoon marks the configuration changed, for a push once the change
// settled.
func (a *App) syncSoon() {
	st := &a.sync
	if st.applied || !st.cfg.Auto || !hylasync.Configured(st.cfg) {
		return
	}
	st.due = time.Now().Add(syncDelay)
}

// syncRun syncs now, way being "" to decide by what both sides did, or
// "push"/"pull".
func (a *App) syncRun(way string) {
	st := &a.sync
	if st.busy || !hylasync.Configured(st.cfg) {
		return
	}
	cfg := st.cfg
	st.busy, st.at, st.due = true, time.Now(), time.Time{}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), syncWait)
		defer cancel()
		res, out, err := hylasync.Run(ctx, cfg, way, machineName())
		if err == nil {
			err = hylasync.Save(out)
		}
		a.update(func() {
			st.busy = false
			if err != nil {
				st.err, st.action = err.Error(), ""
				return
			}
			st.cfg, st.err, st.action, st.host = out, "", res.Action, res.Host
			if res.Action == "pulled" {
				// The configuration the server had is this machine's now:
				// applying it is no change to send back.
				st.applied = true
				a.applySettings(settings.Load())
				st.applied = false
			}
			if res.Backup != "" {
				a.toast(locales.ToastSyncKept(res.Backup))
			}
		})
	}()
}

// syncSave keeps the Sync page's changes, and starts the push they wait for.
func (a *App) syncSave() {
	st := &a.sync
	if err := hylasync.Save(st.cfg); err != nil {
		a.toast(err.Error())
		return
	}
	st.saved, st.loaded, st.err = st.cfg, true, ""
	a.syncSoon()
}

// machineName is this machine, as a commit in a repository names it.
func machineName() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "a machine"
	}
	if i := strings.IndexByte(h, '.'); i > 0 {
		h = h[:i]
	}
	return h
}

// syncLine is what the last sync did, as the Sync page says it.
func (a *App) syncLine() string {
	st := &a.sync
	switch {
	case st.busy:
		return locales.SetSyncRunning()
	case st.err != "":
		return st.err
	}
	// The last attempt this run, else the one the file kept from a start
	// before this.
	at := st.at
	if at.IsZero() && st.cfg.Last.At != 0 {
		at = time.UnixMilli(st.cfg.Last.At)
	}
	if at.IsZero() {
		return locales.SetSyncNever()
	}
	when := ago(at.UnixMilli(), time.Now())
	if st.action == "" {
		return locales.SetSyncLast(when, orMachine(st.cfg.Last.Host))
	}
	switch st.action {
	case "pulled":
		return locales.SetSyncPulled(orMachine(st.host), when)
	case "created", "pushed":
		return locales.SetSyncPushed(when)
	}
	return locales.SetSyncSame(when)
}

func orMachine(host string) string {
	if host == "" {
		return locales.SetSyncAnother()
	}
	return host
}

// syncField is one line of the Sync page's fields: a label, and the text it
// edits, saved as it changes.
type syncField struct {
	key    string
	label  func() string
	value  *string
	secret bool
	place  string
}

// syncPage is Settings › Sync: where this machine's configuration goes, the
// passphrase that seals it, and the button that syncs now.
func (a *App) syncPage(c *ui.Context, pal *theme.Palette) {
	st := &a.sync
	if !st.loaded {
		st.cfg, st.saved, st.loaded = hylasync.Load(), hylasync.Load(), true
	}
	cfg := &st.cfg
	saved := func() { a.syncSave() }
	settingsGroup(c, pal, locales.SetSyncServer(), locales.SetSyncServerDetail(), func() {
		settingRow(c, pal, locales.SetSyncWhere(), locales.SetSyncWhereDetail(), func() {
			segmented(c, "sync-backend", &cfg.Backend, []string{"", "webdav", "s3", "github"},
				[]string{locales.SetSyncOff(), "WebDAV", "S3", "GitHub"})
		})
		var fields []syncField
		switch cfg.Backend {
		case "webdav":
			fields = []syncField{
				{key: "webdav-url", label: locales.SetSyncAddress, value: &cfg.WebDAV.URL, place: "https://dav.jianguoyun.com/dav/"},
				{key: "webdav-user", label: locales.SetSyncUser, value: &cfg.WebDAV.User},
				{key: "webdav-pass", label: locales.SetSyncPassword, value: &cfg.WebDAV.Password, secret: true},
			}
		case "s3":
			fields = []syncField{
				{key: "s3-endpoint", label: locales.SetSyncEndpoint, value: &cfg.S3.Endpoint, place: "https://s3.example.com"},
				{key: "s3-bucket", label: locales.SetSyncBucket, value: &cfg.S3.Bucket},
				{key: "s3-prefix", label: locales.SetSyncPrefix, value: &cfg.S3.Prefix},
				{key: "s3-region", label: locales.SetSyncRegion, value: &cfg.S3.Region},
				{key: "s3-ak", label: locales.SetSyncAccessKey, value: &cfg.S3.AccessKey, secret: true},
				{key: "s3-sk", label: locales.SetSyncSecretKey, value: &cfg.S3.SecretKey, secret: true},
			}
		case "github":
			fields = []syncField{
				{key: "gh-repo", label: locales.SetSyncRepo, value: &cfg.GitHub.Repo, place: "me/hyla-config"},
				{key: "gh-path", label: locales.SetSyncPath, value: &cfg.GitHub.Path, place: "hyla/config.enc"},
				{key: "gh-branch", label: locales.SetSyncBranch, value: &cfg.GitHub.Branch},
				{key: "gh-token", label: locales.SetSyncToken, value: &cfg.GitHub.Token, secret: true},
			}
		}
		for _, f := range fields {
			f := f
			settingRow(c, pal, f.label(), "", func() {
				in := ui.TextInputBase(c.Key(f.key), f.value).Grow(1).MinWidth(0).AlignSelf(ui.Stretch).FontSize(13).
					Placeholder(f.place).OnChange(saved)
				if f.secret {
					in.Password()
				}
			})
		}
	})
	// The controls edit the settings: what changed is saved, and its push
	// waits with the settings' own.
	defer func() {
		if !reflect.DeepEqual(st.cfg, st.saved) {
			a.syncSave()
		}
	}()
	if cfg.Backend == "" {
		return
	}
	settingsGroup(c, pal, locales.SetSyncPassphrase(), locales.SetSyncPassphraseDetail(), func() {
		settingRow(c, pal, locales.SetSyncPassphraseLabel(), "", func() {
			ui.TextInputBase(c.Key("sync-passphrase"), &cfg.Passphrase).Grow(1).MinWidth(0).AlignSelf(ui.Stretch).
				FontSize(13).Password().Placeholder(locales.SetSyncPassphrasePlaceholder()).OnChange(saved)
		})
	})
	settingsGroup(c, pal, locales.SetSyncNowGroup(), "", func() {
		settingRow(c, pal, locales.SetSyncAuto(), locales.SetSyncAutoDetail(), func() {
			style.Toggle(c.Key("sync-auto"), &cfg.Auto, pal).OnChange(saved)
		})
		settingRow(c, pal, locales.SetSyncNow(), a.syncLine(), func() {
			b := ui.Button(c.Key("sync-now"), locales.SetSyncNow()).Padding(5, 12).Radius(7).FontSize(12.5)
			b.Background(pal.Card).TextColor(pal.Text)
			if b.Clicked() && !st.busy {
				a.syncRun("")
			}
		})
	})
}
