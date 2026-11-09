package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/egoist/mygo"

	"hyla/internal/adblock"
	"hyla/internal/proto"
)

// adBlock blocks ads and trackers in browser panes (browser-block-ads):
// the first time a pane shows a page with the setting on, it downloads
// the filter lists the settings name (browser-filter-list), if it has
// to, and compiles them off the main thread, and again when the user
// chooses other lists. Pages made before that is done get the rules when
// it is.
type adBlock struct {
	started bool
	// lists are those compiled, or compiling, last; gen counts the
	// compiles, so that only the latest one's rules apply.
	lists []string
	gen   int
	// failed is how many compiles of those lists failed in a row, and
	// retryAt the earliest the next one may go: a list that is gone for
	// good costs a wait that grows, not a download at every page.
	failed  int
	retryAt time.Time
	rules   []*mygo.ContentRules // main thread
}

// adRetry is how long after a failed compile the next may go, at least,
// and the most it grows to by a list that keeps failing.
const (
	adRetry    = time.Minute
	adRetryMax = 24 * time.Hour
)

// adRules returns the rules browser panes' pages apply: none while the
// setting is off or the rules are not compiled yet.
func (a *App) adRules() []*mygo.ContentRules {
	if !a.set.BlockAds {
		return nil
	}
	a.compileAds()
	return a.ads.rules
}

// compileAds compiles the lists the settings name, unless those compiled
// or compiling last are those: a compile that failed is not one, and the
// next page after a wait tries again. It tells whether it started one.
func (a *App) compileAds() bool {
	if a.ads.started && slices.Equal(a.ads.lists, a.set.AdLists) {
		adblockDone() // none started: what is compiled shows
		return false
	}
	if a.ads.failed > 0 && adNow().Before(a.ads.retryAt) {
		adblockDone() // the lists failed, and their wait is not over
		return false
	}
	if a.ads.lists != nil && !slices.Equal(a.ads.lists, a.set.AdLists) {
		a.ads.failed, a.ads.retryAt = 0, time.Time{}
	}
	a.ads.started, a.ads.lists = true, slices.Clone(a.set.AdLists)
	a.ads.gen++
	gen, lists := a.ads.gen, a.ads.lists
	go func() {
		rules, err := adblockRules(lists)
		if err != nil {
			log.Printf("hyla: ad blocking: %v", err)
		}
		a.main(func() {
			if a.ads.gen != gen {
				adblockDone() // other lists compile now
				return
			}
			if err != nil {
				// As not tried: the rules it did compile, if any, stay,
				// and the next page tries the lists again.
				a.ads.started, a.ads.lists = false, nil
				a.ads.failed++
				wait := adRetry
				for range a.ads.failed - 1 {
					wait *= 2
				}
				a.ads.retryAt = adNow().Add(min(wait, adRetryMax))
				adblockDone() // this compile left no rules to show
				return
			}
			// A success ends the waits, and the lists' that the settings
			// change to go at once, which keep how the last failed.
			a.ads.failed, a.ads.retryAt = 0, time.Time{}
			a.ads.rules = rules
			a.showAdBlock(rules)
			adblockDone()
		})
	}()
	return true
}

var (
	// adblockRules is the lists' rules, a var so that a test need not
	// download them.
	adblockRules = compileAdBlock
	// adblockDone marks each compile's end, which a test's window, with
	// no loop to take the result, waits on.
	adblockDone = func() {}
	// adNow is the time a failed compile's wait ends from, a var so that
	// a test need not wait for the wait.
	adNow = time.Now
)

// applyAdBlock gives the open browser panes' pages the rules, or takes
// them away, as the settings are; they apply to the loads from then on.
func (a *App) applyAdBlock() {
	a.showAdBlock(a.adRules())
}

// showAdBlock gives the pages rules.
func (a *App) showAdBlock(rules []*mygo.ContentRules) {
	for _, e := range a.webs {
		if e.view != nil && !e.view.IsDestroyed() {
			e.view.Page().SetContentRules(rules...)
		}
	}
}

// compiling has one compile run at a time: each compiles the lists
// hyla-ads-0, hyla-ads-1 and so on, and replaces what the one before it
// compiled under those ids.
var compiling sync.Mutex

// compileAdBlock compiles the filter lists of ids, as many content rule
// lists as they take, keeping those it compiled if one fails.
func compileAdBlock(ids []string) ([]*mygo.ContentRules, error) {
	compiling.Lock()
	defer compiling.Unlock()
	var sources []adblock.Source
	for _, id := range ids {
		if s, ok := adblock.Find(id); ok {
			sources = append(sources, s)
		}
	}
	if len(sources) == 0 {
		return nil, nil
	}
	filters, err := adblock.Load(context.Background(), filepath.Join(proto.Dir(), "adblock"), sources)
	if err != nil {
		return nil, err
	}
	var r adblock.Rules
	for _, f := range filters {
		r.Convert(f)
	}
	var rules []*mygo.ContentRules
	for i, list := range r.Lists() {
		cr, cerr := mygo.CompileContentRules(fmt.Sprintf("hyla-ads-%d", i), list)
		if cerr != nil {
			err = cerr
			continue
		}
		rules = append(rules, cr)
	}
	return rules, err
}
