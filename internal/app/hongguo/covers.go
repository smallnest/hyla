package hongguo

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
)

const (
	coverMaxBytes    = 8 << 20
	coverRetryWait   = 5 * time.Minute
	coverMaxMemory   = 512
	coverConcurrency = 4
)

// CoverCache fetches, converts (HEIC -> JPEG), disk-caches, and decodes drama poster bitmaps.
type CoverCache struct {
	dir        string
	sem        chan struct{}
	httpClient *http.Client
	findFFmpeg func() (string, error)

	mu      sync.Mutex
	bitmaps map[string]*ui.Bitmap
	order   []string
	pending map[string]bool
	failed  map[string]time.Time
}

// NewCoverCache creates a CoverCache storing converted JPEG posters in dir.
func NewCoverCache(dir string) *CoverCache {
	return &CoverCache{
		dir:        dir,
		sem:        make(chan struct{}, coverConcurrency),
		httpClient: &http.Client{Timeout: 20 * time.Second},
		findFFmpeg: FindFFmpeg,
		bitmaps:    make(map[string]*ui.Bitmap),
		pending:    make(map[string]bool),
		failed:     make(map[string]time.Time),
	}
}

// Get returns the decoded bitmap for rawURL if already cached; otherwise it schedules
// an asynchronous fetch/decode and invokes onReady once complete.
func (c *CoverCache) Get(rawURL string, onReady func()) *ui.Bitmap {
	if rawURL == "" {
		return nil
	}
	c.mu.Lock()
	if bm, ok := c.bitmaps[rawURL]; ok {
		for i, u := range c.order {
			if u == rawURL {
				if i < len(c.order)-1 {
					c.order = append(append(c.order[:i], c.order[i+1:]...), rawURL)
				}
				break
			}
		}
		c.mu.Unlock()
		return bm
	}
	if at, ok := c.failed[rawURL]; ok && time.Since(at) < coverRetryWait {
		c.mu.Unlock()
		return nil
	}
	if c.pending[rawURL] {
		c.mu.Unlock()
		return nil
	}
	c.pending[rawURL] = true
	c.mu.Unlock()

	go c.load(rawURL, onReady)
	return nil
}

func (c *CoverCache) load(rawURL string, onReady func()) {
	defer func() {
		c.mu.Lock()
		delete(c.pending, rawURL)
		c.mu.Unlock()
	}()

	c.sem <- struct{}{}
	defer func() { <-c.sem }()

	bm, err := c.fetchOrConvert(rawURL)
	c.mu.Lock()
	if err != nil || bm == nil {
		c.failed[rawURL] = time.Now()
		c.mu.Unlock()
		return
	}
	delete(c.failed, rawURL)
	if _, exists := c.bitmaps[rawURL]; !exists {
		c.bitmaps[rawURL] = bm
		c.order = append(c.order, rawURL)
		for len(c.order) > coverMaxMemory {
			evict := c.order[0]
			c.order = c.order[1:]
			delete(c.bitmaps, evict)
		}
	}
	c.mu.Unlock()

	if onReady != nil {
		onReady()
	}
}

func (c *CoverCache) fetchOrConvert(rawURL string) (*ui.Bitmap, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errors.New("invalid cover URL")
	}

	sum := sha1.Sum([]byte(rawURL))
	key := hex.EncodeToString(sum[:])
	var cachedPath string
	if c.dir != "" {
		cachedPath = filepath.Join(c.dir, key+".jpg")
		if b, err := os.ReadFile(cachedPath); err == nil && len(b) > 0 {
			if bm, err := ui.DecodeBitmap(b); err == nil {
				return bm, nil
			}
			_ = os.Remove(cachedPath)
		}
	}

	data, err := c.download(rawURL)
	if err != nil {
		return nil, err
	}
	if bm, err := ui.DecodeBitmap(data); err == nil {
		if cachedPath != "" {
			_ = os.MkdirAll(c.dir, 0o700)
			_ = writeAtomic(cachedPath, data)
		}
		return bm, nil
	}

	// Hongguo CDN serves HEIC posters which Go's image decoders don't decode natively.
	if c.dir == "" {
		return nil, errors.New("cannot convert cover without cache directory")
	}
	if err := os.MkdirAll(c.dir, 0o700); err != nil {
		return nil, err
	}
	rawPath := filepath.Join(c.dir, fmt.Sprintf("%s.%d.raw", key, time.Now().UnixNano()))
	tmpJPG := filepath.Join(c.dir, fmt.Sprintf("%s.%d.jpg", key, time.Now().UnixNano()))
	defer func() {
		_ = os.Remove(rawPath)
		_ = os.Remove(tmpJPG)
	}()
	if err := os.WriteFile(rawPath, data, 0o600); err != nil {
		return nil, err
	}
	if err := c.convertCover(rawPath, tmpJPG); err != nil {
		return nil, err
	}
	converted, err := os.ReadFile(tmpJPG)
	if err != nil {
		return nil, err
	}
	bm, err := ui.DecodeBitmap(converted)
	if err != nil {
		return nil, err
	}
	_ = os.Rename(tmpJPG, cachedPath)
	return bm, nil
}

func (c *CoverCache) download(rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := range 2 {
		req, err := http.NewRequest(http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt == 0 {
				time.Sleep(300 * time.Millisecond)
			}
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, coverMaxBytes+1))
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("cover HTTP %d", resp.StatusCode)
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				return nil, lastErr
			}
			continue
		}
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if len(body) > coverMaxBytes {
			return nil, errors.New("cover too large")
		}
		return body, nil
	}
	if lastErr == nil {
		return nil, errors.New("cover download failed")
	}
	return nil, lastErr
}

func (c *CoverCache) convertCover(rawPath, outPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if runtime.GOOS == "darwin" {
		cmd := exec.CommandContext(ctx, "sips", "-s", "format", "jpeg", "-s", "formatOptions", "70", "--resampleWidth", "240", rawPath, "--out", outPath)
		if out, err := cmd.CombinedOutput(); err == nil {
			return nil
		} else if c.findFFmpeg == nil {
			return fmt.Errorf("sips failed: %v (%s)", err, string(out))
		}
	}
	if c.findFFmpeg == nil {
		return ErrFFmpegMissing
	}
	bin, err := c.findFFmpeg()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, bin, "-v", "error", "-y", "-i", rawPath, "-frames:v", "1", "-vf", "scale=240:-2", "-q:v", "6", "-update", "1", outPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg cover failed: %v (%s)", err, string(out))
	}
	return nil
}
