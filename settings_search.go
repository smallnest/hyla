package main

import (
	"hyla/internal/ui/settings/search"
)

// searchEntry is a setting the Settings window's search finds: its page,
// its label as its row shows it, and words that find it besides, as its
// key in the configuration file.
type searchEntry struct {
	page, label, words string
}

// settingsPagesOf are the Settings' pages, as the search knows them.
func settingsPagesOf() []search.Page {
	pages := make([]search.Page, 0, len(settingsPages))
	for _, p := range settingsPages {
		pages = append(pages, search.Page{ID: p.id, Title: p.title()})
	}
	return pages
}

// searchResults returns what a query finds, by page.
func searchResults(query string, shortcuts map[string]string) map[string][]string {
	return search.Results(query, shortcuts, settingsPagesOf(), actionTitle)
}
