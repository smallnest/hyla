package manifest

// IsBuiltin reports whether id is a bundled built-in plugin.
func IsBuiltin(id string) bool {
	return isBuiltinEntry(id)
}

// IsEnabled reports whether plugin id is present (installed, linked, or bundled)
// and currently enabled without pending installation.
func (s *Store) IsEnabled(id string) bool {
	if s == nil {
		s = DefaultStore()
	}
	list, err := s.Discover()
	if err != nil {
		return false
	}
	for _, p := range list {
		if p.Entry.ID == id {
			return p.Entry.Enabled && !p.Entry.PendingInstall
		}
	}
	return false
}
