// Package webicon is the icons browser panes show in their address bars.
package webicon

import (
	"context"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/app/favicon"
)

// Site is a browser pane's page, as the icons ask it: where it is, the
// page it shows, and the icon URL it took.
type Site struct {
	URL     string
	Page    Page
	IconURL string
}

// Page is a page, as the icons ask it which icons it has.
type Page interface {
	EvalContext(ctx context.Context, script string) (any, error)
}

// App is what the icons ask of the app.
type App interface {
	// Icons is the cache the icons are kept in.
	Icons() *favicon.Cache
	// Update runs fn on the main thread.
	Update(fn func())
}

// Icon returns the icon of a browser pane's page: the one it told, else
// its site's last, which it reads from the disk the first time.
func Icon(x App, s *Site) ui.ImageSource {
	c := x.Icons()
	if i := c.Get(s.IconURL); s.IconURL != "" && i != nil {
		return i
	}
	origin := favicon.SiteOrigin(s.URL)
	if origin == "" {
		return nil
	}
	if u, ok := c.Site(origin); ok {
		return c.Get(u)
	}
	if c.Reading(origin) {
		return nil
	}
	c.SetReading(origin, true)
	go func() {
		u, img := favicon.ReadFromDisk(origin)
		x.Update(func() {
			c.SetReading(origin, false)
			if _, ok := c.Site(origin); ok {
				return // the page told its icon meanwhile
			}
			c.SetSite(origin, u, img)
		})
	}()
	return nil
}

// Found asks the page which icons it has, best first, and shows the first
// that decodes, downloading it unless it is in memory and fresh.
func Found(x App, s *Site) {
	if s.Page == nil || favicon.SiteOrigin(s.URL) == "" {
		return
	}
	page, at := s.Page, s.URL
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		v, err := page.EvalContext(ctx, favicon.Script)
		if err != nil {
			return
		}
		list, _ := v.([]any)
		var urls []string
		for _, u := range list {
			if s, ok := u.(string); ok && s != "" {
				urls = append(urls, s)
			}
		}
		if len(urls) == 0 {
			return
		}
		x.Update(func() {
			if s.URL != at {
				return // the page went elsewhere
			}
			use(x, s, urls)
		})
	}()
}

// use shows the first of urls that is in memory and fresh, else downloads
// them, and shows what decodes.
func use(x App, s *Site, urls []string) {
	c := x.Icons()
	origin := favicon.SiteOrigin(s.URL)
	for _, u := range urls {
		if i := c.Get(u); i != nil && c.Fresh(u) {
			s.IconURL = u
			if u0, _ := c.Site(origin); u0 != u {
				c.SetSite(origin, u, i)
				favicon.WriteToDisk(origin, u, c.Data(u))
			}
			return
		}
	}
	key := strings.Join(urls, "\n")
	if c.Fetching(key) {
		return
	}
	c.SetFetching(key, true)
	go func() {
		u, data, img := favicon.Fetch(urls)
		if img != nil {
			favicon.WriteToDisk(origin, u, data)
		}
		x.Update(func() {
			// Icons that fail stay unasked for until the next launch.
			if img == nil {
				return
			}
			c.SetFetching(key, false)
			c.Add(u, img, data)
			c.SetSite(origin, u, img)
			if favicon.SiteOrigin(s.URL) == origin {
				s.IconURL = u
			}
		})
	}()
}
