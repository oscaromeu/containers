// Package allquiet is a minimal client for the AllQuiet public API.
// Read-only for now: the sync only lists and reads incidents. The triage
// TUI will add the write intents (Affects/Archive) to this same package.
package allquiet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultBaseURL = "https://allquiet.eu"

var (
	ErrEmptyAPIKey       = errors.New("api key must not be empty")
	ErrInvalidHTTPClient = errors.New("http client must not be nil")
)

// Client talks to one AllQuiet region (EU by default).
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// Option is an optional client setting.
type Option func(*Client) error

// WithHTTPClient sets a custom http client.
func WithHTTPClient(c *http.Client) Option {
	return func(cl *Client) error {
		if c == nil {
			return ErrInvalidHTTPClient
		}
		cl.http = c
		return nil
	}
}

// WithBaseURL points the client at another region or a test server.
func WithBaseURL(u string) Option {
	return func(cl *Client) error {
		cl.baseURL = strings.TrimRight(u, "/")
		return nil
	}
}

// New returns a client authenticated with the given API key.
func New(apiKey string, options ...Option) (*Client, error) {
	if apiKey == "" {
		return nil, ErrEmptyAPIKey
	}
	c := &Client{
		baseURL: defaultBaseURL,
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
	for _, option := range options {
		if err := option(c); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	u := c.baseURL + "/api/public/v1" + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Authorization", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("GET %s: %s: %s", path, resp.Status, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
