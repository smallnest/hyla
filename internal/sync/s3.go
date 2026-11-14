package sync

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// s3Store is an object in an S3-compatible bucket: hyla/config.enc under the
// prefix, signed with the version 4 signature every such service takes.
type s3Store struct {
	endpoint  string // https://s3.example.com, without its trailing slash
	bucket    string
	key       string
	region    string
	accessKey string
	secretKey string
	pathStyle bool
	client    *http.Client
}

func newS3(c S3) (*s3Store, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(c.Endpoint), "/")
	if endpoint == "" || strings.TrimSpace(c.Bucket) == "" {
		return nil, errors.New("sync: the S3 endpoint and bucket want filling")
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}
	if c.AccessKey == "" || c.SecretKey == "" {
		return nil, errors.New("sync: the S3 access key and secret want filling")
	}
	prefix := strings.Trim(c.Prefix, "/")
	key := "hyla/config.enc"
	if prefix != "" {
		key = prefix + "/" + key
	}
	return &s3Store{
		endpoint:  endpoint,
		bucket:    strings.TrimSpace(c.Bucket),
		key:       key,
		region:    or(c.Region, "us-east-1"),
		accessKey: c.AccessKey,
		secretKey: c.SecretKey,
		pathStyle: c.PathStyle,
		client:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (s *s3Store) url() string {
	key := escapeKey(s.key)
	if s.pathStyle {
		return s.endpoint + "/" + s.bucket + "/" + key
	}
	u, err := url.Parse(s.endpoint)
	if err != nil {
		return s.endpoint + "/" + s.bucket + "/" + key
	}
	u.Host = s.bucket + "." + u.Host
	u.Path = "/" + key
	return u.String()
}

// escapeKey escapes a key's segments, keeping the slashes between them.
func escapeKey(key string) string {
	parts := strings.Split(key, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

// Get reads the snapshot, or ErrNotFound on a bucket that has none.
func (s *s3Store) Get(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url(), nil)
	if err != nil {
		return nil, err
	}
	s.sign(req, nil)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, ErrNotFound
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("sync: S3 GET %s: %s (%s)", s.url(), resp.Status, s3Message(body))
	}
	return io.ReadAll(resp.Body)
}

// Put writes the snapshot whole.
func (s *s3Store) Put(ctx context.Context, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, s.url(), bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	s.sign(req, data)
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("sync: S3 PUT %s: %s (%s)", s.url(), resp.Status, s3Message(body))
	}
	return nil
}

// s3Message is what an S3 service says went wrong, as its XML holds it.
func s3Message(body []byte) string {
	var m struct {
		Code    string `xml:"Code"`
		Message string `xml:"Message"`
	}
	if xml.Unmarshal(body, &m) == nil && (m.Code != "" || m.Message != "") {
		return strings.TrimSpace(m.Code + " " + m.Message)
	}
	return strings.TrimSpace(string(body))
}

// sign signs the request as version 4 of AWS's signature does, which every
// S3-compatible service takes: the payload's hash is part of what is signed,
// so a body changed on the way fails.
func (s *s3Store) sign(req *http.Request, payload []byte) {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	date := amzDate[:8]
	sum := sha256.Sum256(payload)
	payloadHash := hex.EncodeToString(sum[:])
	req.Header.Set("x-amz-content-sha256", payloadHash)
	req.Header.Set("x-amz-date", amzDate)

	host := req.URL.Host
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalHeaders := "host:" + host + "\n" +
		"x-amz-content-sha256:" + payloadHash + "\n" +
		"x-amz-date:" + amzDate + "\n"
	canonical := strings.Join([]string{
		req.Method,
		req.URL.EscapedPath(),
		req.URL.RawQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	scope := date + "/" + s.region + "/s3/aws4_request"
	toSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		hexSum([]byte(canonical)),
	}, "\n")

	key := hmacSum([]byte("AWS4"+s.secretKey), date)
	key = hmacSum(key, s.region)
	key = hmacSum(key, "s3")
	key = hmacSum(key, "aws4_request")
	sig := hex.EncodeToString(hmacSum(key, toSign))

	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.accessKey, scope, signedHeaders, sig))
}

func hexSum(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func hmacSum(key []byte, data string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(data))
	return m.Sum(nil)
}
