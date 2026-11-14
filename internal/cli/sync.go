package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"hyla/internal/sync"
)

// syncCmd is `hyla sync`: this machine's configuration, kept the same as the
// other machines' through the server the Settings window names.
func syncCmd(args []string) error {
	f, err := parse(args, nil, nil)
	if err != nil {
		return err
	}
	c := sync.Load()
	sub := ""
	if len(f.pos) > 0 {
		sub = f.pos[0]
	}
	switch sub {
	case "", "now":
		return syncRun(c, "")
	case "push", "pull":
		return syncRun(c, sub)
	case "status":
		return syncStatus(c)
	case "set":
		if len(f.pos) < 3 {
			return usageErr("hyla sync set <key> <value>")
		}
		return syncSet(c, f.pos[1], strings.Join(f.pos[2:], " "))
	case "off":
		c.Backend = ""
		if err := sync.Save(c); err != nil {
			return err
		}
		fmt.Println("sync is off")
		return nil
	}
	return usageErr("hyla sync [now|status|push|pull|set <key> <value>|off]")
}

// syncRun syncs, and says what it did. way is "" to decide by what both sides
// did since the last sync.
func syncRun(c sync.Config, way string) error {
	if !sync.Configured(c) {
		return errors.New("no server is set: fill one in Settings › Sync, or hyla sync set backend webdav …")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	res, out, err := sync.Run(ctx, c, way, machineName())
	if err != nil {
		return err
	}
	if err := sync.Save(out); err != nil {
		return err
	}
	switch res.Action {
	case "unchanged":
		fmt.Println("unchanged: both sides have the configuration " + shortHash(out.Last.Hash))
	case "created":
		fmt.Println("created the snapshot on the server")
	case "pushed":
		fmt.Printf("pushed this machine's configuration, %s\n", shortHash(out.Last.Hash))
	case "pulled":
		fmt.Printf("pulled %s's configuration, %s\n", orServer(res.Host), shortHash(out.Last.Hash))
	}
	if res.Backup != "" {
		fmt.Println("kept the server's own in " + res.Backup)
	}
	return nil
}

// syncStatus says where this machine syncs, and where it last got to.
func syncStatus(c sync.Config) error {
	if c.Backend == "" {
		fmt.Println("sync: off")
		return nil
	}
	fmt.Println("sync:", c.Backend, syncAddress(c))
	if !sync.Configured(c) {
		fmt.Println("  incomplete: the Settings window names what is missing")
	}
	auto := "off"
	if c.Auto {
		auto = "on"
	}
	fmt.Println("  auto:", auto)
	if c.Last.At == 0 {
		fmt.Println("  last: never")
		return nil
	}
	fmt.Printf("  last: %s from %s, %s (%s)\n", syncAgo(c.Last.At), orServer(c.Last.Host),
		time.UnixMilli(c.Last.At).Format("2006-01-02 15:04"), shortHash(c.Last.Hash))
	if c.Last.Backup != "" {
		fmt.Println("  kept the server's own in " + c.Last.Backup)
	}
	return nil
}

// syncSet fills one of the sync settings, by its dotted name.
func syncSet(c sync.Config, key, value string) error {
	switch key {
	case "backend":
		switch value {
		case "webdav", "s3", "github", "":
			c.Backend = value
		case "off":
			c.Backend = ""
		default:
			return usageErr("sync: unknown server %q, want webdav, s3 or github", value)
		}
	case "passphrase":
		c.Passphrase = value
	case "auto":
		c.Auto = value == "on" || value == "true" || value == "1"
	case "webdav.url":
		c.WebDAV.URL = value
	case "webdav.user":
		c.WebDAV.User = value
	case "webdav.password":
		c.WebDAV.Password = value
	case "s3.endpoint":
		c.S3.Endpoint = value
	case "s3.bucket":
		c.S3.Bucket = value
	case "s3.prefix":
		c.S3.Prefix = value
	case "s3.region":
		c.S3.Region = value
	case "s3.accessKey":
		c.S3.AccessKey = value
	case "s3.secretKey":
		c.S3.SecretKey = value
	case "s3.pathStyle":
		c.S3.PathStyle = value == "on" || value == "true" || value == "1"
	case "github.repo":
		c.GitHub.Repo = value
	case "github.path":
		c.GitHub.Path = value
	case "github.branch":
		c.GitHub.Branch = value
	case "github.token":
		c.GitHub.Token = value
	default:
		return usageErr("sync: unknown key %q", key)
	}
	if err := sync.Save(c); err != nil {
		return err
	}
	fmt.Println("ok:", sync.Path())
	return nil
}

// syncAddress is the server c names, as status tells it.
func syncAddress(c sync.Config) string {
	switch c.Backend {
	case "webdav":
		return c.WebDAV.URL
	case "s3":
		return c.S3.Bucket + " at " + c.S3.Endpoint
	case "github":
		return c.GitHub.Repo + ":" + c.GitHub.Path
	}
	return ""
}

// machineName is this machine, as a commit in a repository names it.
func machineName() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "a machine"
	}
	if i := strings.IndexByte(h, '.'); i > 0 {
		h = h[:i]
	}
	return h
}

func orServer(host string) string {
	if host == "" {
		return "another machine"
	}
	return host
}

func shortHash(h string) string {
	if len(h) > 8 {
		return h[:8]
	}
	return h
}

// syncAgo is how long ago a sync was, as status tells it.
func syncAgo(ms int64) string {
	d := time.Since(time.UnixMilli(ms))
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	}
	return fmt.Sprintf("%d days ago", int(d.Hours()/24))
}
