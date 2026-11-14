package sync

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// githubStore is a file in a repository, kept with GitHub's contents API: a
// private repository, for a configuration that holds secrets.
type githubStore struct {
	api    string // https://api.github.com
	repo   string // owner/name
	path   string // hyla/config.enc
	branch string // "" for the repository's own
	token  string
	host   string // the machine, which the commit names
	client *http.Client
}

func newGitHub(c GitHub) (*githubStore, error) {
	repo := strings.Trim(strings.TrimSpace(c.Repo), "/")
	if strings.Count(repo, "/") != 1 {
		return nil, fmt.Errorf("sync: the repository %q wants owner/name", c.Repo)
	}
	if strings.TrimSpace(c.Token) == "" {
		return nil, errors.New("sync: the GitHub token is empty")
	}
	return &githubStore{
		api:    "https://api.github.com",
		repo:   repo,
		path:   or(c.Path, "hyla/config.enc"),
		branch: strings.TrimSpace(c.Branch),
		token:  c.Token,
		client: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (s *githubStore) url() string {
	u := s.api + "/repos/" + s.repo + "/contents/" + escapeKey(s.path)
	if s.branch != "" {
		return u + "?ref=" + url.QueryEscape(s.branch)
	}
	return u
}

func (s *githubStore) do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "Hyla")
	return s.client.Do(req)
}

// githubFile is what the contents API says about a file: its bytes in base64,
// and the blob's id, which replacing it wants.
type githubFile struct {
	Content string `json:"content"`
	SHA     string `json:"sha"`
	Message string `json:"message"`
}

// Get reads the snapshot, or ErrNotFound on a repository that has none.
func (s *githubStore) Get(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, ErrNotFound
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		return nil, fmt.Errorf("sync: GitHub %s: %s", resp.Status, githubMessage(body))
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		return nil, fmt.Errorf("sync: GitHub GET %s: %s", resp.Status, githubMessage(body))
	}
	var f githubFile
	if err := json.Unmarshal(body, &f); err != nil {
		return nil, fmt.Errorf("sync: GitHub's answer does not read: %w", err)
	}
	// The API wraps base64 in lines, which the decoder would stop at.
	return base64.StdEncoding.DecodeString(strings.ReplaceAll(f.Content, "\n", ""))
}

// Put writes the snapshot whole, as one commit.
func (s *githubStore) Put(ctx context.Context, data []byte) error {
	sha, err := s.sha(ctx)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	body := map[string]any{
		"message": "hyla: configuration from " + or(s.host, "a machine"),
		"content": base64.StdEncoding.EncodeToString(data),
	}
	if s.branch != "" {
		body["branch"] = s.branch
	}
	if sha != "" {
		body["sha"] = sha
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	// Without the ref the branch is the body's: the API reads it there for a
	// write.
	req, err := http.NewRequestWithContext(ctx, http.MethodPut,
		s.api+"/repos/"+s.repo+"/contents/"+escapeKey(s.path), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	answer, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("sync: GitHub PUT %s: %s", resp.Status, githubMessage(answer))
	}
	return nil
}

// sha is the file's, which replacing it wants, or ErrNotFound for one the
// repository has not.
func (s *githubStore) sha(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url(), nil)
	if err != nil {
		return "", err
	}
	resp, err := s.do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return "", ErrNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("sync: GitHub GET %s: %s", resp.Status, githubMessage(body))
	}
	var f githubFile
	if err := json.Unmarshal(body, &f); err != nil {
		return "", fmt.Errorf("sync: GitHub's answer does not read: %w", err)
	}
	return f.SHA, nil
}

// githubMessage is what GitHub says went wrong, as its answer holds it.
func githubMessage(body []byte) string {
	var m githubFile
	if json.Unmarshal(body, &m) == nil && m.Message != "" {
		return m.Message
	}
	return strings.TrimSpace(string(body))
}
