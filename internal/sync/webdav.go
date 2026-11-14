package sync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// webdavStore is a WebDAV folder: hyla/config.enc within the address the
// user gave, which the first push makes.
type webdavStore struct {
	base   string // the address, without its trailing slash
	user   string
	pass   string
	client *http.Client
}

func newWebDAV(c WebDAV) (*webdavStore, error) {
	addr := strings.TrimSpace(c.URL)
	if addr == "" {
		return nil, errors.New("sync: the WebDAV address is empty")
	}
	if !strings.Contains(addr, "://") {
		return nil, fmt.Errorf("sync: the WebDAV address %q wants a scheme, as https://", addr)
	}
	return &webdavStore{
		base:   strings.TrimRight(addr, "/"),
		user:   c.User,
		pass:   c.Password,
		client: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (s *webdavStore) dir() string { return s.base + "/hyla/" }
func (s *webdavStore) url() string { return s.base + "/hyla/config.enc" }

func (s *webdavStore) do(req *http.Request) (*http.Response, error) {
	if s.user != "" || s.pass != "" {
		req.SetBasicAuth(s.user, s.pass)
	}
	req.Header.Set("User-Agent", "Hyla")
	return s.client.Do(req)
}

// Get reads the snapshot, or ErrNotFound on a folder that has none.
func (s *webdavStore) Get(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, ErrNotFound
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		return nil, fmt.Errorf("sync: WebDAV GET %s: %s", s.url(), resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// Put writes the snapshot whole, making the folder when it is not there.
func (s *webdavStore) Put(ctx context.Context, data []byte) error {
	if err := s.mkcol(ctx); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, s.url(), bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("sync: WebDAV PUT %s: %s", s.url(), resp.Status)
	}
	return nil
}

// mkcol makes the hyla folder, unless the server says it was there: 405 is a
// folder that exists, which is what the second push meets.
func (s *webdavStore) mkcol(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "MKCOL", s.dir(), nil)
	if err != nil {
		return err
	}
	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusMethodNotAllowed:
		return nil
	}
	return fmt.Errorf("sync: WebDAV MKCOL %s: %s", s.dir(), resp.Status)
}
