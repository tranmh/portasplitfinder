package source

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ErrBlocked is returned by the HTTP client when a server responds with a status
// that indicates the request was rejected/blocked rather than a transient
// failure: 403 (Forbidden), 404 (Not Found, which OBI returns to non-browser
// clients), or 429 (Too Many Requests). Callers should treat it as "this source
// is bot-protected / unavailable right now" and degrade gracefully.
var ErrBlocked = errors.New("source: request blocked")

// defaultUserAgent is a browser-like User-Agent. Many German retailers (OBI in
// particular) reject non-browser clients, so real adapters present as a browser.
const defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
	"(KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// Client is a small HTTP helper shared by the real source adapters. It sends
// browser-like headers, retries transient failures (5xx and transport/timeout
// errors) with exponential backoff, and maps blocking statuses to ErrBlocked.
//
// The zero value is not ready to use; build one with NewClient.
type Client struct {
	hc          *http.Client
	userAgent   string
	acceptLang  string
	maxAttempts int
	// baseBackoff is the first backoff interval; each retry doubles it. Set to 0
	// in tests to avoid real sleeps.
	baseBackoff time.Duration
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithTimeout sets the per-request timeout (default 15s).
func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) { c.hc.Timeout = d }
}

// WithMaxAttempts sets the total number of attempts, including the first
// (default 3). Values < 1 are clamped to 1.
func WithMaxAttempts(n int) ClientOption {
	return func(c *Client) {
		if n < 1 {
			n = 1
		}
		c.maxAttempts = n
	}
}

// WithBaseBackoff sets the initial backoff between retries (default 200ms). Pass
// 0 in tests to retry without sleeping.
func WithBaseBackoff(d time.Duration) ClientOption {
	return func(c *Client) { c.baseBackoff = d }
}

// WithHTTPClient overrides the underlying *http.Client (tests).
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.hc = hc }
}

// NewClient builds a Client with browser-like defaults.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		hc:          &http.Client{Timeout: 15 * time.Second},
		userAgent:   defaultUserAgent,
		acceptLang:  "de-DE,de;q=0.9,en;q=0.8",
		maxAttempts: 3,
		baseBackoff: 200 * time.Millisecond,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// GetHTML fetches url and returns the response body. On 403/404/429 it returns
// ErrBlocked. It retries 5xx responses and transport/timeout errors up to
// maxAttempts with exponential backoff; the final error is returned if all
// attempts fail.
func (c *Client) GetHTML(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	backoff := c.baseBackoff

	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		body, retryable, err := c.try(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		// ErrBlocked and other non-retryable errors fail immediately.
		if !retryable {
			return nil, err
		}
		// Last attempt: don't sleep, just return.
		if attempt == c.maxAttempts {
			break
		}
		if backoff > 0 {
			t := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				t.Stop()
				return nil, ctx.Err()
			case <-t.C:
			}
		} else if err := ctx.Err(); err != nil {
			return nil, err
		}
		backoff *= 2
	}
	return nil, lastErr
}

// try performs a single request. retryable reports whether the error is worth
// retrying (5xx or transport/timeout).
func (c *Client) try(ctx context.Context, url string) (body []byte, retryable bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept-Language", c.acceptLang)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := c.hc.Do(req)
	if err != nil {
		// Transport/timeout errors are retryable.
		return nil, true, fmt.Errorf("source: get %s: %w", url, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusForbidden,
		resp.StatusCode == http.StatusNotFound,
		resp.StatusCode == http.StatusTooManyRequests:
		return nil, false, fmt.Errorf("%w: %s returned %d", ErrBlocked, url, resp.StatusCode)
	case resp.StatusCode >= 500:
		return nil, true, fmt.Errorf("source: %s returned %d", url, resp.StatusCode)
	case resp.StatusCode >= 400:
		return nil, false, fmt.Errorf("source: %s returned %d", url, resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, fmt.Errorf("source: read body %s: %w", url, err)
	}
	return b, false, nil
}
