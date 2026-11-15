// Package sync keeps this machine's configuration in step with the
// others', by the server the Sync page names: it pulls at launch, and
// pushes a change once it waited.
package sync

import (
	"context"
	"os"
	"strings"
	"time"

	"hyla/internal/settings"
	hylasync "hyla/internal/sync"
)

// Delay is how long a change waits before it goes up: the controls save
// as they change, and a run of them is one push.
const Delay = 30 * time.Second

// Wait bounds a sync: a server that does not answer in that time leaves
// the next one to try.
const Wait = 90 * time.Second

// State is what the app knows of syncing this machine's configuration:
// the server it syncs to, as the Sync page last saved it, and what the
// last sync did.
type State struct {
	// Cfg is the server and the settings it carries, and Saved the same
	// as the file has it, for what changed.
	Cfg   hylasync.Config
	Saved hylasync.Config
	// Loaded tells that Cfg and Saved came from the file, and Busy that
	// a sync runs.
	Loaded bool
	Busy   bool
	// Pulled tells that the launch pull is done, and Applied that the
	// settings being applied are a pull's, not a change.
	Pulled  bool
	Applied bool
	// Due is when a change's push waits for, and At when the last attempt
	// was; Action and Host say what it did, and Err what went wrong.
	Due    time.Time
	At     time.Time
	Action string
	Host   string
	Err    string
}

// Hosts is what the state reads of the app: whether its state came, and
// what it does with a message and with settings a pull brought.
type Hosts interface {
	// StateCame reports whether a state came from hylad.
	StateCame() bool
	// Update runs fn on the main thread, and draws.
	Update(fn func())
	// Toast shows a line to the user.
	Toast(msg string)
	// ToastSyncKept says that the server's configuration before a pull is
	// kept, at path.
	ToastSyncKept(path string)
	// ApplySettings makes a pull's settings the app's.
	ApplySettings(s settings.Settings)
}

// Ensure reads the configuration from the file, once.
func (st *State) Ensure() {
	if !st.Loaded {
		st.Cfg, st.Saved, st.Loaded = hylasync.Load(), hylasync.Load(), true
	}
}

// Tick pulls at launch, and pushes a change once it waited.
func (st *State) Tick(h Hosts) {
	st.Ensure()
	if !hylasync.Configured(st.Cfg) || st.Busy {
		return
	}
	if !st.Pulled && h.StateCame() {
		st.Pulled = true
		st.Run(h, "")
		return
	}
	if st.Due.IsZero() || time.Now().Before(st.Due) {
		return
	}
	st.Due = time.Time{}
	st.Run(h, "")
}

// Soon marks the configuration changed, for a push once the change
// settled.
func (st *State) Soon() {
	if st.Applied || !st.Cfg.Auto || !hylasync.Configured(st.Cfg) {
		return
	}
	st.Due = time.Now().Add(Delay)
}

// Run syncs now, way being "" to decide by what both sides did, or
// "push"/"pull".
func (st *State) Run(h Hosts, way string) {
	if st.Busy || !hylasync.Configured(st.Cfg) {
		return
	}
	cfg := st.Cfg
	st.Busy, st.At, st.Due = true, time.Now(), time.Time{}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), Wait)
		defer cancel()
		res, out, err := hylasync.Run(ctx, cfg, way, MachineName())
		if err == nil {
			err = hylasync.Save(out)
		}
		h.Update(func() {
			st.Busy = false
			if err != nil {
				st.Err, st.Action = err.Error(), ""
				return
			}
			st.Cfg, st.Err, st.Action, st.Host = out, "", res.Action, res.Host
			if res.Action == "pulled" {
				// The configuration the server had is this machine's now:
				// applying it is no change to send back.
				st.Applied = true
				h.ApplySettings(settings.Load())
				st.Applied = false
			}
			if res.Backup != "" {
				h.ToastSyncKept(res.Backup)
			}
		})
	}()
}

// Save keeps the Sync page's changes, and starts the push they wait for.
// A file that does not take them is h.Toast's to say.
func (st *State) Save(h Hosts) bool {
	if err := hylasync.Save(st.Cfg); err != nil {
		h.Toast(err.Error())
		return false
	}
	st.Saved, st.Loaded, st.Err = st.Cfg, true, ""
	st.Soon()
	return true
}

// MachineName is this machine, as a commit in a repository names it.
func MachineName() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "a machine"
	}
	if i := strings.IndexByte(h, '.'); i > 0 {
		h = h[:i]
	}
	return h
}
