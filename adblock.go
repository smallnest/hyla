package main

import (
	"github.com/egoist/mygo"

	"hyla/internal/proto"
)

// adRules returns the rules browser panes' pages apply: none while the
// setting is off or the rules are not compiled yet.
func (a *App) adRules() []*mygo.ContentRules {
	if !a.settingsIn.BlockAds {
		return nil
	}
	return a.Ads.Rules(a.settingsIn.BlockAds, a.settingsIn.AdLists, proto.Dir(), a.main, a.showAdBlock)
}

// applyAdBlock gives the open browser panes' pages the rules, or takes
// them away, as the settings are; they apply to the loads from then on.
func (a *App) applyAdBlock() {
	a.showAdBlock(a.adRules())
}

// showAdBlock gives the pages rules.
func (a *App) showAdBlock(rules []*mygo.ContentRules) {
	for _, e := range a.Webs {
		if e.view != nil && !e.view.IsDestroyed() {
			e.view.Page().SetContentRules(rules...)
		}
	}
}
