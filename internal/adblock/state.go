package adblock

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

// State blocks ads and trackers in browser panes (browser-block-ads): the
// first time a pane shows a page with the setting on, it downloads the
// filter lists the settings name (browser-filter-list), if it has to, and
// compiles them off the main thread, and again when the user chooses
// other lists. Pages made before that is done get the rules when it is.
type State struct {
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

// Retry is how long after a failed compile the next may go, at least,
// and RetryMax the most it grows to by a list that keeps failing.
const (
	Retry    = time.Minute
	RetryMax = 24 * time.Hour
)

// Started tells that a compile of the lists the settings name began, or
// that one compiled: what the settings' changes need to know.
func (s *State) Started() bool { return s.started }

// Rules returns the rules browser panes' pages apply: none while the
// setting is off or the rules are not compiled yet. main runs fn on the
// main thread, as compileAds' goroutine, which takes the compile's result,
// needs it to; show gives the pages the rules.
func (s *State) Rules(on bool, lists []string, dir string, main func(func()), show func([]*mygo.ContentRules)) []*mygo.ContentRules {
	if !on {
		return nil
	}
	s.compile(lists, dir, main, show)
	return s.rules
}

// compile compiles the lists, unless those compiled or compiling last are
// those: a compile that failed is not one, and the next page after a wait
// tries again. It tells whether it started one.
func (s *State) compile(lists []string, dir string, main func(func()), show func([]*mygo.ContentRules)) bool {
	if s.started && slices.Equal(s.lists, lists) {
		compiled() // none started: what is compiled shows
		return false
	}
	if s.failed > 0 && now().Before(s.retryAt) {
		compiled() // the lists failed, and their wait is not over
		return false
	}
	if s.lists != nil && !slices.Equal(s.lists, lists) {
		s.failed, s.retryAt = 0, time.Time{}
	}
	s.started, s.lists = true, slices.Clone(lists)
	s.gen++
	gen, ls := s.gen, s.lists
	go func() {
		rules, err := compileRules(ls, dir)
		if err != nil {
			log.Printf("hyla: ad blocking: %v", err)
		}
		main(func() {
			if s.gen != gen {
				compiled() // other lists compile now
				return
			}
			if err != nil {
				// As not tried: the rules it did compile, if any, stay,
				// and the next page tries the lists again.
				s.started, s.lists = false, nil
				s.failed++
				wait := Retry
				for range s.failed - 1 {
					wait *= 2
				}
				s.retryAt = now().Add(min(wait, RetryMax))
				compiled() // this compile left no rules to show
				return
			}
			// A success ends the waits, and the lists' that the settings
			// change to go at once, which keep how the last failed.
			s.failed, s.retryAt = 0, time.Time{}
			s.rules = rules
			show(rules)
			compiled()
		})
	}()
	return true
}

var (
	// compileRules is the lists' rules, a var so that a test need not
	// download them.
	compileRules = Compile
	// compiled marks each compile's end, which a test's window, with no
	// loop to take the result, waits on.
	compiled = func() {}
	// now is the time a failed compile's wait ends from, a var so that a
	// test need not wait for the wait.
	now = time.Now
)

// Hook is what a test stands in for: the compile, what marks its end, and
// the clock a failed compile's wait counts from. Set returns the hooks to,
// as the test's cleanup.
func Hook(compile func(lists []string, dir string) ([]*mygo.ContentRules, error), done func(), clock func() time.Time) (undo func()) {
	compileRules, compiled, now = compile, done, clock
	return func() { compileRules, compiled, now = Compile, func() {}, time.Now }
}

// compiling has one compile run at a time: each compiles the lists
// hyla-ads-0, hyla-ads-1 and so on, and replaces what the one before it
// compiled under those ids.
var compiling sync.Mutex

// Compile compiles the filter lists of ids, as many content rule lists as
// they take, keeping those it compiled if one fails. It keeps the lists
// in dir.
func Compile(ids []string, dir string) ([]*mygo.ContentRules, error) {
	compiling.Lock()
	defer compiling.Unlock()
	var sources []Source
	for _, id := range ids {
		if s, ok := Find(id); ok {
			sources = append(sources, s)
		}
	}
	if len(sources) == 0 {
		return nil, nil
	}
	filters, err := Load(context.Background(), filepath.Join(dir, "adblock"), sources)
	if err != nil {
		return nil, err
	}
	var r Rules
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
