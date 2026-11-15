package main

import (
	"reflect"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/ui/style"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

// syncLine is what the last sync did, as the Sync page says it.
func (a *App) syncLine() string {
	st := &a.State
	switch {
	case st.Busy:
		return locales.SetSyncRunning()
	case st.Err != "":
		return st.Err
	}
	// The last attempt this run, else the one the file kept from a start
	// before this.
	at := st.At
	if at.IsZero() && st.Cfg.Last.At != 0 {
		at = time.UnixMilli(st.Cfg.Last.At)
	}
	if at.IsZero() {
		return locales.SetSyncNever()
	}
	when := ago(at.UnixMilli(), time.Now())
	if st.Action == "" {
		return locales.SetSyncLast(when, orMachine(st.Cfg.Last.Host))
	}
	switch st.Action {
	case "pulled":
		return locales.SetSyncPulled(orMachine(st.Host), when)
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
	st := &a.State
	st.Ensure()
	cfg := &st.Cfg
	saved := func() { st.Save(a) }
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
		if !reflect.DeepEqual(st.Cfg, st.Saved) {
			st.Save(a)
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
			if b.Clicked() && !st.Busy {
				st.Run(a, "")
			}
		})
	})
}
