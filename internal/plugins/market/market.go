// Package market discovers, parses, verifies, and installs plugins from
// Hyla's plugin marketplace catalog (plugin_market.json).
package market

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"slices"
	"strings"
	"time"

	"hyla/internal/netproxy"
	"hyla/internal/plugins/manifest"
	"hyla/internal/update"
)

//go:embed plugin_market.json
var embeddedPluginMarket []byte

// DefaultMarketURL points to the marketplace index in Hyla's main repository.
const DefaultMarketURL = "https://raw.githubusercontent.com/" + update.Repo + "/master/plugin_market.json"

const (
	maxCatalogBytes  = 4 << 20
	maxPackageBytes  = manifest.MaxPackageSingleFileBytes
	maxChecksumBytes = 4096
)

// MarketURL returns the marketplace catalog URL, honoring HYLA_PLUGIN_MARKET_URL when set.
func MarketURL() string {
	if u := strings.TrimSpace(os.Getenv("HYLA_PLUGIN_MARKET_URL")); u != "" {
		return u
	}
	return DefaultMarketURL
}

// Catalog represents the root plugin_market.json document.
type Catalog struct {
	Version int      `json:"version"`
	Plugins []Plugin `json:"plugins"`
}

// PluginLocale holds localized display metadata for a marketplace entry.
type PluginLocale struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// DownloadTarget specifies a downloadable plugin archive and optional checksum.
type DownloadTarget struct {
	URL       string `json:"url"`
	SHA256    string `json:"sha256,omitempty"`
	SHA256URL string `json:"sha256_url,omitempty"`
}

// Plugin describes a single plugin entry in plugin_market.json.
type Plugin struct {
	ID             string                    `json:"id"`
	Name           string                    `json:"name"`
	Author         string                    `json:"author,omitempty"`
	Description    string                    `json:"description,omitempty"`
	Homepage       string                    `json:"homepage,omitempty"`
	Version        string                    `json:"version"`
	MinHylaVersion string                    `json:"min_hyla_version,omitempty"`
	Permissions    []string                  `json:"permissions,omitempty"`
	I18n           map[string]PluginLocale   `json:"i18n,omitempty"`
	DownloadURL    string                    `json:"download_url,omitempty"`
	SHA256         string                    `json:"sha256,omitempty"`
	SHA256URL      string                    `json:"sha256_url,omitempty"`
	Downloads      map[string]DownloadTarget `json:"downloads,omitempty"`
}

// Parse decodes and validates raw plugin_market.json bytes.
func Parse(data []byte) (*Catalog, error) {
	var cat Catalog
	if err := json.Unmarshal(data, &cat); err != nil {
		return nil, fmt.Errorf("parse plugin market: %w", err)
	}
	if cat.Version != 1 {
		return nil, fmt.Errorf("unsupported plugin market version %d (supported: 1)", cat.Version)
	}
	seen := make(map[string]bool, len(cat.Plugins))
	for i := range cat.Plugins {
		p := &cat.Plugins[i]
		if err := manifest.ValidateID(p.ID); err != nil {
			return nil, fmt.Errorf("market plugin[%d]: %w", i, err)
		}
		if seen[p.ID] {
			return nil, fmt.Errorf("duplicate market plugin id %q", p.ID)
		}
		seen[p.ID] = true
		if strings.TrimSpace(p.Name) == "" {
			return nil, fmt.Errorf("market plugin %q: name must not be empty", p.ID)
		}
		if strings.TrimSpace(p.Version) == "" {
			return nil, fmt.Errorf("market plugin %q: version must not be empty", p.ID)
		}
		if p.Homepage != "" {
			if err := validateHTTPURL(p.Homepage); err != nil {
				return nil, fmt.Errorf("market plugin %q homepage: %w", p.ID, err)
			}
		}
		for _, sc := range p.Permissions {
			if !manifest.IsKnownPermission(sc) {
				return nil, fmt.Errorf("market plugin %q: unknown permission %q", p.ID, sc)
			}
		}
		if err := validatePluginDownloads(*p); err != nil {
			return nil, err
		}
	}
	return &cat, nil
}

func validatePluginDownloads(p Plugin) error {
	if strings.TrimSpace(p.DownloadURL) == "" && len(p.Downloads) == 0 {
		return fmt.Errorf("market plugin %q: missing download_url or downloads", p.ID)
	}
	if p.DownloadURL != "" {
		sample := expandDownloadPlaceholders(p.DownloadURL, p.ID, p.Version, "linux", "amd64")
		if err := validateHTTPURL(sample); err != nil {
			return fmt.Errorf("market plugin %q download_url: %w", p.ID, err)
		}
	}
	if p.SHA256 != "" && !isValidSHA256Hex(p.SHA256) {
		return fmt.Errorf("market plugin %q sha256: invalid hex digest", p.ID)
	}
	if p.SHA256URL != "" {
		sample := expandDownloadPlaceholders(p.SHA256URL, p.ID, p.Version, "linux", "amd64")
		if err := validateHTTPURL(sample); err != nil {
			return fmt.Errorf("market plugin %q sha256_url: %w", p.ID, err)
		}
	}
	for plat, tgt := range p.Downloads {
		if strings.TrimSpace(tgt.URL) == "" {
			return fmt.Errorf("market plugin %q downloads[%q]: empty url", p.ID, plat)
		}
		sample := expandDownloadPlaceholders(tgt.URL, p.ID, p.Version, "linux", "amd64")
		if err := validateHTTPURL(sample); err != nil {
			return fmt.Errorf("market plugin %q downloads[%q]: %w", p.ID, plat, err)
		}
		if tgt.SHA256 != "" && !isValidSHA256Hex(tgt.SHA256) {
			return fmt.Errorf("market plugin %q downloads[%q] sha256: invalid hex digest", p.ID, plat)
		}
		if tgt.SHA256URL != "" {
			s := expandDownloadPlaceholders(tgt.SHA256URL, p.ID, p.Version, "linux", "amd64")
			if err := validateHTTPURL(s); err != nil {
				return fmt.Errorf("market plugin %q downloads[%q] sha256_url: %w", p.ID, plat, err)
			}
		}
	}
	return nil
}

// Embedded parses Hyla's bundled fallback plugin_market.json catalog.
func Embedded() (*Catalog, error) {
	return Parse(embeddedPluginMarket)
}

// InLanguage returns a shallow copy of p with Name and Description localized for locale.
func (p Plugin) InLanguage(locale string) Plugin {
	if len(p.I18n) == 0 {
		return p
	}
	cands := manifest.LocaleCandidates(locale)
	if len(cands) == 0 {
		return p
	}
	byNorm := make(map[string]PluginLocale, len(p.I18n))
	for k, v := range p.I18n {
		norm := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(k, "_", "-")))
		if norm != "" {
			byNorm[norm] = v
		}
	}
	for _, cand := range cands {
		if tr, ok := byNorm[cand]; ok {
			if tr.Name != "" {
				p.Name = tr.Name
			}
			if tr.Description != "" {
				p.Description = tr.Description
			}
			break
		}
	}
	return p
}

// InLanguage returns a localized copy of c for locale without mutating c.
func (c *Catalog) InLanguage(locale string) *Catalog {
	if c == nil {
		return nil
	}
	out := &Catalog{
		Version: c.Version,
		Plugins: make([]Plugin, len(c.Plugins)),
	}
	for i, p := range c.Plugins {
		out.Plugins[i] = p.InLanguage(locale)
	}
	return out
}

// ResolveDownload picks and expands the download target for goos/goarch.
func (p Plugin) ResolveDownload(goos, goarch string) (DownloadTarget, error) {
	var raw DownloadTarget
	found := false
	for _, key := range []string{goos + "/" + goarch, goos + "-" + goarch, "any"} {
		if tgt, ok := p.Downloads[key]; ok && strings.TrimSpace(tgt.URL) != "" {
			raw = tgt
			found = true
			break
		}
	}
	if !found && strings.TrimSpace(p.DownloadURL) != "" {
		raw = DownloadTarget{
			URL:       p.DownloadURL,
			SHA256:    p.SHA256,
			SHA256URL: p.SHA256URL,
		}
		found = true
	}
	if !found {
		return DownloadTarget{}, fmt.Errorf("plugin %q has no download available for %s/%s", p.ID, goos, goarch)
	}
	res := DownloadTarget{
		URL:       expandDownloadPlaceholders(raw.URL, p.ID, p.Version, goos, goarch),
		SHA256:    strings.ToLower(strings.TrimSpace(raw.SHA256)),
		SHA256URL: expandDownloadPlaceholders(raw.SHA256URL, p.ID, p.Version, goos, goarch),
	}
	if err := validateHTTPURL(res.URL); err != nil {
		return DownloadTarget{}, err
	}
	if res.SHA256URL != "" {
		if err := validateHTTPURL(res.SHA256URL); err != nil {
			return DownloadTarget{}, err
		}
	}
	if res.SHA256 != "" && !isValidSHA256Hex(res.SHA256) {
		return DownloadTarget{}, fmt.Errorf("invalid sha256 digest for %s", p.ID)
	}
	return res, nil
}

// HasUpdate reports whether marketVersion is strictly newer than installedVersion.
func HasUpdate(installedVersion, marketVersion string) bool {
	installedVersion = strings.TrimSpace(installedVersion)
	marketVersion = strings.TrimSpace(marketVersion)
	if installedVersion == "" || marketVersion == "" {
		return false
	}
	return update.Compare(marketVersion, installedVersion) > 0
}

func expandDownloadPlaceholders(raw, id, version, goos, goarch string) string {
	r := strings.NewReplacer(
		"{id}", id,
		"{version}", version,
		"{os}", goos,
		"{arch}", goarch,
	)
	return strings.TrimSpace(r.Replace(raw))
}

func validateHTTPURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return err
	}
	if !strings.EqualFold(u.Scheme, "https") && !strings.EqualFold(u.Scheme, "http") {
		return fmt.Errorf("unsupported URL scheme %q (expected http or https)", u.Scheme)
	}
	if strings.TrimSpace(u.Host) == "" {
		return fmt.Errorf("URL %q missing host", raw)
	}
	return nil
}

func isValidSHA256Hex(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// marketTimeout bounds a request of the market that a caller leaves without
// a deadline: an install waits for a package to come over a network Hyla
// does not own.
const marketTimeout = 30 * time.Minute

// Client fetches the marketplace catalog and downloads verified plugin packages.
type Client struct {
	HTTP *http.Client
	URL  string
}

func (c *Client) httpClient() *http.Client {
	if c != nil && c.HTTP != nil {
		return c.HTTP
	}
	// A package may be large and the network slow: the client waits as long
	// as an install is given, and the caller's context ends a wait that is
	// over. Sixty seconds ended installs of a package on a slow line. It
	// goes through the system's proxy, as a browser does.
	return &http.Client{Timeout: marketTimeout, Transport: netproxy.Transport()}
}

func (c *Client) url() string {
	if c != nil && strings.TrimSpace(c.URL) != "" {
		return strings.TrimSpace(c.URL)
	}
	return MarketURL()
}

// Fetch retrieves and validates the remote marketplace catalog.
func (c *Client) Fetch(ctx context.Context) (*Catalog, error) {
	targetURL := c.url()
	if err := validateHTTPURL(targetURL); err != nil {
		return nil, err
	}
	b, err := c.getBytes(ctx, targetURL, maxCatalogBytes)
	if err != nil {
		return nil, err
	}
	return Parse(b)
}

// FetchWithFallback tries Fetch and falls back to Embedded() when remote fetch fails.
// fromRemote reports whether the returned catalog came from the network.
func (c *Client) FetchWithFallback(ctx context.Context) (cat *Catalog, fromRemote bool, fetchErr error) {
	cat, fetchErr = c.Fetch(ctx)
	if fetchErr == nil {
		return cat, true, nil
	}
	emb, embErr := Embedded()
	if embErr == nil {
		return emb, false, fetchErr
	}
	return nil, false, fetchErr
}

// DownloadAndInstall downloads p for goos/goarch, verifies its checksum and plugin ID,
// and installs it into store while enabling it and granting p.Permissions.
func (c *Client) DownloadAndInstall(ctx context.Context, store *manifest.Store, p Plugin, goos, goarch, currentHylaVersion string) (*manifest.InstalledPlugin, error) {
	if store == nil {
		store = manifest.DefaultStore()
	}
	if req := strings.TrimSpace(p.MinHylaVersion); req != "" && currentHylaVersion != "" {
		if !strings.HasPrefix(req, ">") && !strings.HasPrefix(req, "<") && !strings.HasPrefix(req, "=") && !strings.HasPrefix(req, "^") {
			req = ">=" + req
		}
		compatCheck := &manifest.Manifest{
			ID:      p.ID,
			Engines: manifest.EnginesSpec{Hyla: req},
		}
		if err := compatCheck.CompatibleWith(currentHylaVersion); err != nil {
			return nil, err
		}
	}

	dl, err := p.ResolveDownload(goos, goarch)
	if err != nil {
		return nil, err
	}

	tmp, err := os.CreateTemp("", "hyla-market-*.hylapkg")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(tmpPath)

	gotSHA, err := c.downloadFile(ctx, dl.URL, tmpPath)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", p.ID, err)
	}

	wantSHA := dl.SHA256
	if wantSHA == "" && dl.SHA256URL != "" {
		sumBytes, err := c.getBytes(ctx, dl.SHA256URL, maxChecksumBytes)
		if err != nil {
			return nil, fmt.Errorf("download %s checksum: %w", p.ID, err)
		}
		wantFile := path.Base(dl.URL)
		parsed, err := parseSHA256Text(sumBytes, wantFile)
		if err != nil {
			return nil, fmt.Errorf("parse %s checksum: %w", p.ID, err)
		}
		wantSHA = parsed
	}
	if wantSHA != "" && !strings.EqualFold(gotSHA, wantSHA) {
		return nil, fmt.Errorf("checksum mismatch for %s: got %s, want %s", p.ID, gotSHA, wantSHA)
	}

	if err := verifyPackageID(tmpPath, p.ID); err != nil {
		return nil, err
	}

	return store.InstallPackage(tmpPath, manifest.InstallOptions{
		Enable:           true,
		GrantPermissions: slices.Clone(p.Permissions),
		CurrentVersion:   currentHylaVersion,
	})
}

func (c *Client) getBytes(ctx context.Context, rawURL string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Hyla-Plugin-Market/1")
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", rawURL, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("%s: response too large", rawURL)
	}
	return b, nil
}

func (c *Client) downloadFile(ctx context.Context, rawURL, dst string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Hyla-Plugin-Market/1")
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", rawURL, resp.Status)
	}
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, maxPackageBytes+1))
	closeErr := f.Close()
	if copyErr != nil {
		return "", copyErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	if n > maxPackageBytes {
		return "", fmt.Errorf("%s: package exceeds size limit (%d bytes)", rawURL, maxPackageBytes)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func parseSHA256Text(data []byte, wantFile string) (string, error) {
	var firstHex string
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		sum := strings.ToLower(fields[0])
		if !isValidSHA256Hex(sum) {
			continue
		}
		if len(fields) == 1 {
			if firstHex == "" {
				firstHex = sum
			}
			continue
		}
		fileName := strings.TrimPrefix(fields[len(fields)-1], "*")
		if wantFile != "" && path.Base(strings.ReplaceAll(fileName, `\`, "/")) == wantFile {
			return sum, nil
		}
		if firstHex == "" {
			firstHex = sum
		}
	}
	if firstHex == "" {
		return "", errors.New("no valid sha256 digest found")
	}
	return firstHex, nil
}

func verifyPackageID(pkgPath, expectedID string) error {
	zr, err := zip.OpenReader(pkgPath)
	if err != nil {
		return fmt.Errorf("open package: %w", err)
	}
	defer zr.Close()

	var rootEntry *zip.File
	var nestedEntries []*zip.File
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		clean := strings.TrimPrefix(strings.ReplaceAll(f.Name, `\`, "/"), "./")
		if clean == manifest.ManifestFileName {
			rootEntry = f
			break
		}
		if strings.Count(clean, "/") == 1 && strings.HasSuffix(clean, "/"+manifest.ManifestFileName) {
			nestedEntries = append(nestedEntries, f)
		}
	}
	target := rootEntry
	if target == nil && len(nestedEntries) == 1 {
		target = nestedEntries[0]
	}
	if target == nil {
		return fmt.Errorf("package missing %s", manifest.ManifestFileName)
	}
	rc, err := target.Open()
	if err != nil {
		return err
	}
	b, err := io.ReadAll(io.LimitReader(rc, (1<<20)+1))
	_ = rc.Close()
	if err != nil {
		return err
	}
	if len(b) > 1<<20 {
		return fmt.Errorf("%s exceeds 1 MiB", manifest.ManifestFileName)
	}
	m, err := manifest.Parse(b)
	if err != nil {
		return err
	}
	if m.ID != expectedID {
		return fmt.Errorf("package plugin id %q does not match marketplace entry %q", m.ID, expectedID)
	}
	return nil
}
