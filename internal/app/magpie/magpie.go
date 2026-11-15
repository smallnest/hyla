// Package magpie reads Magpie's usage: a coding agents' gateway that runs
// on this machine, on port 3425, and keeps a file of what they spent.
package magpie

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// pages bounds the reads of its requests, which one session's calls
	// may lie deep in, and Wait keeps them off the redraws: a read that
	// takes that long is off the table until it answers.
	pages = 16
	Wait  = time.Minute
)

// Usage is Magpie's usage as the Info view shows it.
type Usage struct {
	Today       Bucket
	Session     int  // this pane's conversation's calls, 0 if another
	SessErr     bool // its calls are past the pages read
	SessionCost float64
	Latest      Call
	Windows     []Window // the plan's, closest to its limit first
}

// Call is the latest call through the gateway.
type Call struct {
	Status int
	Served string
	Ms     int
}

// Window is a plan's window, as left.
type Window struct {
	Provider string // the provider's name, as Magpie names it
	Key      string // the provider's key, as an agent's name: "antigravity"
	Name     string
	Left     float64 // percent
}

// Bucket is calls' count, tokens and cost, as Magpie keeps them.
type Bucket struct {
	Calls      int     `json:"calls"`
	Errors     int     `json:"errors"`
	Input      int     `json:"input"`
	Output     int     `json:"output"`
	CacheRead  int     `json:"cache_read"`
	CacheWrite int     `json:"cache_write"`
	Reasoning  int     `json:"reasoning"`
	Cost       float64 `json:"cost"`
}

// Tokens counts the bucket's tokens: those it read with the cache, as read
// and as written, and wrote.
func (b Bucket) Count() int {
	return b.Input + b.Output + b.CacheRead + b.CacheWrite + b.Reasoning
}

// HitShare is the bucket's share of its input that the cache read, in percent.
func (b Bucket) HitShare() float64 {
	if in := b.Input + b.CacheRead + b.CacheWrite; in > 0 {
		return float64(b.CacheRead) * 100 / float64(in)
	}
	return 0
}

// URL is where Magpie answers: MAGPIE_URL, else its port, else its
// own.
func URL() string {
	if u := strings.TrimSpace(os.Getenv("MAGPIE_URL")); u != "" {
		return strings.TrimRight(u, "/")
	}
	port := "3425"
	if p := strings.TrimSpace(os.Getenv("MAGPIE_PORT")); p != "" {
		port = p
	}
	return "http://127.0.0.1:" + port
}

// Fetch reads Magpie at base: its usage, the calls of session and
// the plan windows. The session's calls are the requests, which the
// daemon of another machine does not answer: they come from this
// machine's usage file alone.
func Fetch(base, session string) (*Usage, error) {
	now := time.Now()
	if !Runs(base) {
		return nil, errors.New("magpie does not run at " + base)
	}
	var usage usageJSON
	if err := get(base+"/v1/magpie/usage", &usage); err != nil {
		return nil, err
	}
	u := &Usage{Today: usage.Bucket}
	if s := usage.Series; len(s) > 0 {
		if day := s[len(s)-1].Time; day != "" {
			if d, err := time.Parse(time.RFC3339, day); err == nil && sameDay(d, now) {
				u.Today = s[len(s)-1].Bucket
			}
		}
	}
	// The windows come next: a gateway whose plans it does not read yet is
	// still a gateway.
	var quotas quotasJSON
	if err := get(base+"/v1/magpie/quotas", &quotas); err == nil {
		u.Windows = mostLeft(quotas)
	}
	var reqs struct {
		requestsJSON
		Since string `json:"since"`
	}
	if err := get(base+"/v1/magpie/usage/requests?limit=1", &reqs); err != nil || len(reqs.Rows) == 0 {
		return u, nil
	}
	l := reqs.Rows[0]
	u.Latest = Call{Status: l.Status, Served: l.Served, Ms: l.Ms}
	if session == "" {
		return u, nil
	}
	if l.Session == session && reqs.Total == 1 {
		u.Session, u.SessionCost = 1, l.Cost
		return u, nil
	}
	// The requests come newest first: a page that reaches today's start
	// holds every call of it.
	since, err := time.Parse(time.RFC3339, reqs.Since)
	if err != nil {
		since = startOfDay(now)
	}
	for page, offset := 0, 0; page < pages; page, offset = page+1, offset+500 {
		var r requestsJSON
		url := base + "/v1/magpie/usage/requests?limit=500&offset=" + strconv.Itoa(offset)
		if err := get(url, &r); err != nil {
			u.SessErr = true
			return u, nil
		}
		for _, row := range r.Rows {
			if row.Session != session {
				continue
			}
			if t, err := time.Parse(time.RFC3339, row.T); err == nil && t.After(since) {
				u.Session++
				u.SessionCost += row.Cost
			}
		}
		if len(r.Rows) < 500 || offset+500 >= r.Total {
			return u, nil
		}
		if last := r.Rows[len(r.Rows)-1]; last.T != "" {
			if t, err := time.Parse(time.RFC3339, last.T); err == nil && t.Before(since) {
				return u, nil
			}
		}
	}
	u.SessErr = true
	return u, nil
}

// Runs reports whether Magpie answers at base.
func Runs(base string) bool {
	var root struct {
		Name string `json:"name"`
	}
	if err := get(base+"/", &root); err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(root.Name), "magpie")
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// get reads url's JSON into v.
func get(url string, v any) error {
	client := http.Client{Timeout: 4 * time.Second}
	res, err := client.Get(url)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, res.Status)
	}
	return json.NewDecoder(io.LimitReader(res.Body, 16<<20)).Decode(v)
}

// mostLeft picks the plan's windows closest to their limits: the
// fewest left first.
func mostLeft(q quotasJSON) []Window {
	var all []Window
	for _, p := range q.Data {
		for _, w := range p.Windows {
			if w.Unlimited {
				continue
			}
			all = append(all, Window{Provider: p.Name, Key: p.Provider, Name: w.Name, Left: w.Remaining})
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].Left < all[j].Left })
	return all
}

type usageJSON struct {
	Bucket
	Series []struct {
		Bucket
		Time string `json:"time"`
	} `json:"series"`
}

type requestsJSON struct {
	Total int `json:"total"`
	Rows  []struct {
		T       string  `json:"t"`
		Session string  `json:"session"`
		Status  int     `json:"status"`
		Served  string  `json:"served"`
		Ms      int     `json:"ms"`
		Cost    float64 `json:"cost"`
	} `json:"rows"`
}

type quotasJSON struct {
	Data []struct {
		Name     string `json:"name"`
		Provider string `json:"provider"`
		Windows  []struct {
			Name      string  `json:"name"`
			Remaining float64 `json:"remaining"`
			Unlimited bool    `json:"unlimited"`
		} `json:"windows"`
	} `json:"data"`
}

// Left is a window's share left in words.
func Left(left float64) string {
	return strconv.Itoa(int(math.Round(left))) + "%"
}

// Hit is how much of today's input the cache read.
func Hit(b Bucket) string {
	if b.Input+b.CacheRead+b.CacheWrite == 0 {
		return "—"
	}
	return strconv.Itoa(int(math.Round(b.HitShare()))) + "%"
}

// Cost is dollars in words: $1.18, $0.042, $0.0006.
func Cost(c float64) string {
	switch {
	case c >= 100:
		return "$" + strconv.Itoa(int(math.Round(c)))
	case c >= 10:
		return "$" + strconv.FormatFloat(c, 'f', 1, 64)
	case c >= 1:
		return "$" + strconv.FormatFloat(c, 'f', 2, 64)
	case c >= 0.01:
		return "$" + strconv.FormatFloat(c, 'f', 3, 64)
	case c > 0:
		return "$" + strconv.FormatFloat(c, 'f', 4, 64)
	}
	return "$0"
}

// Tokens is a count of tokens in short words: 1.2M, 34.5k, 812.
func Tokens(n int) string {
	switch {
	case n >= 10_000_000:
		return strconv.Itoa(n/1_000_000) + "M"
	case n >= 1_000_000:
		return strconv.FormatFloat(float64(n)/1e6, 'f', 1, 64) + "M"
	case n >= 100_000:
		return strconv.Itoa(n/1_000) + "k"
	case n >= 1_000:
		return strconv.FormatFloat(float64(n)/1e3, 'f', 1, 64) + "k"
	}
	return strconv.Itoa(n)
}

// CallLine is the latest call: its status, what served it and how
// long it took.
func CallLine(l Call) string {
	line := strconv.Itoa(l.Status) + " " + l.Served
	if l.Ms > 0 {
		line += " · " + FmtTime(l.Ms)
	}
	return line
}

// FmtTime is a duration in short words: 1.4s, 840ms.
func FmtTime(ms int) string {
	if ms < 1000 {
		return strconv.Itoa(ms) + "ms"
	}
	return strconv.FormatFloat(float64(ms)/1000, 'f', 1, 64) + "s"
}
