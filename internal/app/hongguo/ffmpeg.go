package hongguo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

// ErrFFmpegMissing reports that no ffmpeg executable was found to decrypt protected streams.
var ErrFFmpegMissing = errors.New("hongguo: ffmpeg not found")

var mediaHTTPClient = &http.Client{Timeout: 60 * time.Second}

// FindFFmpeg locates a usable ffmpeg binary on the host machine.
func FindFFmpeg() (string, error) {
	var candidates []string
	if runtime.GOOS == "windows" {
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			candidates = append(candidates, filepath.Join(localAppData, "Microsoft", "WinGet", "Links", "ffmpeg.exe"))
		}
		if programFiles := os.Getenv("ProgramFiles"); programFiles != "" {
			candidates = append(candidates, filepath.Join(programFiles, "ffmpeg", "bin", "ffmpeg.exe"))
		}
	} else {
		candidates = append(candidates, "/opt/homebrew/bin/ffmpeg", "/usr/local/bin/ffmpeg", "/usr/bin/ffmpeg")
	}
	for _, p := range candidates {
		if p != "" {
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p, nil
			}
		}
	}
	if bin, err := exec.LookPath("ffmpeg"); err == nil {
		return bin, nil
	}
	return "", ErrFFmpegMissing
}

// DecryptMP4 downloads an encrypted MP4 stream and remuxes it into a plain faststart MP4
// using ffmpeg without re-encoding. The decryption key is never included in error messages.
func DecryptMP4(ctx context.Context, ffmpegBin, keyHex, srcURL, backupURL, outPath string) error {
	if st, err := os.Stat(outPath); err == nil && st.Size() > 0 {
		now := time.Now()
		_ = os.Chtimes(outPath, now, now)
		return nil
	}
	if ffmpegBin == "" {
		return ErrFFmpegMissing
	}
	if keyHex == "" {
		return errors.New("hongguo: missing content key")
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o700); err != nil {
		return err
	}

	encPath := fmt.Sprintf("%s.%d.part.enc", outPath, time.Now().UnixNano())
	partMP4 := fmt.Sprintf("%s.%d.part.mp4", outPath, time.Now().UnixNano())
	defer func() {
		_ = os.Remove(encPath)
		_ = os.Remove(partMP4)
	}()

	if err := downloadEncrypted(ctx, srcURL, backupURL, encPath); err != nil {
		return err
	}

	ffCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(
		ffCtx,
		ffmpegBin,
		"-v", "error",
		"-y",
		"-decryption_key", keyHex,
		"-i", encPath,
		"-map", "0",
		"-c", "copy",
		"-tag:v", "hvc1",
		"-movflags", "+faststart",
		partMP4,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		msg = strings.ReplaceAll(msg, keyHex, "[redacted]")
		if msg != "" {
			return fmt.Errorf("ffmpeg decrypt failed: %s", msg)
		}
		return fmt.Errorf("ffmpeg decrypt failed: %w", err)
	}
	if st, err := os.Stat(partMP4); err != nil || st.Size() == 0 {
		return errors.New("ffmpeg produced an empty file")
	}
	return os.Rename(partMP4, outPath)
}

func downloadEncrypted(ctx context.Context, srcURL, backupURL, dstPath string) error {
	urls := []string{srcURL}
	if backupURL != "" && backupURL != srcURL {
		urls = append(urls, backupURL)
	}
	var lastErr error
	for _, rawURL := range urls {
		u, err := url.Parse(rawURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			lastErr = errors.New("invalid stream URL")
			continue
		}
		for attempt := range 3 {
			if err := ctx.Err(); err != nil {
				return err
			}
			status, dlErr := fetchFile(ctx, rawURL, dstPath)
			if dlErr == nil {
				return nil
			}
			lastErr = dlErr
			if status >= 400 && status < 500 && status != http.StatusTooManyRequests {
				break
			}
			if attempt < 2 {
				wait := time.Duration(800*(attempt+1)) * time.Millisecond
				t := time.NewTimer(wait)
				select {
				case <-ctx.Done():
					t.Stop()
					return ctx.Err()
				case <-t.C:
				}
			}
		}
	}
	if lastErr == nil {
		return errors.New("download failed")
	}
	return lastErr
}

func fetchFile(ctx context.Context, rawURL, dstPath string) (int, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, err
	}
	resp, err := mediaHTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("CDN HTTP %d", resp.StatusCode)
	}
	f, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return resp.StatusCode, err
	}
	_, cpErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if cpErr != nil {
		return resp.StatusCode, cpErr
	}
	if closeErr != nil {
		return resp.StatusCode, closeErr
	}
	return resp.StatusCode, nil
}

// PruneMedia deletes older cached .mp4 files in dir so at most keep remain.
func PruneMedia(dir string, keep int) {
	if keep <= 0 || dir == "" {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type item struct {
		path string
		mod  time.Time
	}
	var files []item
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".mp4") || strings.Contains(name, ".part") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, item{path: filepath.Join(dir, name), mod: info.ModTime()})
	}
	if len(files) <= keep {
		return
	}
	slices.SortFunc(files, func(a, b item) int {
		return b.mod.Compare(a.mod)
	})
	for _, f := range files[keep:] {
		_ = os.Remove(f.path)
	}
}
