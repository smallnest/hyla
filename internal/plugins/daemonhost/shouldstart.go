package daemonhost

import (
	"sync"

	"hyla/internal/plugins/manifest"
)

// BuiltinShouldStartFunc reports whether a built-in plugin should auto-start
// during Manager.Reload (for instance, a KeepAlive channel plugin only starts
// automatically once configured).
type BuiltinShouldStartFunc func(st *manifest.Store) bool

var (
	builtinShouldStartMu sync.RWMutex
	builtinShouldStarts  = map[string]BuiltinShouldStartFunc{}
)

// RegisterBuiltinShouldStart registers a startup predicate for a built-in plugin.
func RegisterBuiltinShouldStart(pluginID, hostKind string, fn BuiltinShouldStartFunc) {
	if pluginID == "" || hostKind == "" || fn == nil {
		return
	}
	builtinShouldStartMu.Lock()
	builtinShouldStarts[pluginID+"|"+hostKind] = fn
	builtinShouldStartMu.Unlock()
}

func getBuiltinShouldStart(pluginID, hostKind string) BuiltinShouldStartFunc {
	builtinShouldStartMu.RLock()
	fn := builtinShouldStarts[pluginID+"|"+hostKind]
	builtinShouldStartMu.RUnlock()
	return fn
}

func (m *Manager) shouldAutoStart(p manifest.InstalledPlugin, spec *manifest.HostProcessSpec) bool {
	if spec == nil || (!spec.AutoStart && !spec.KeepAlive) {
		return false
	}
	if p.Builtin {
		if fn := getBuiltinShouldStart(p.Entry.ID, m.opts.HostKind); fn != nil {
			return fn(m.opts.Store)
		}
	}
	return true
}
