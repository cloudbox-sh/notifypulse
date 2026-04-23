// Package client is a thin HTTP wrapper around the Notifypulse REST API.
//
// Authentication is API-key-based (X-API-Key header) against the /v1 API
// for day-to-day calls, plus a short-lived session cookie used once during
// login to bootstrap a CLI key through /api/keys.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"time"
)

// Client is the Notifypulse API client. Zero value is not usable — use New.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
	// cookies are used only by login — session-auth calls to /api/* to
	// bootstrap an API key and then clean up the session.
	cookieJar http.CookieJar
}

// New returns a client pointed at baseURL, authenticating with apiKey.
// An empty apiKey is allowed — login endpoints work unauthenticated.
func New(baseURL, apiKey string) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		apiKey:    apiKey,
		cookieJar: jar,
		http: &http.Client{
			Timeout: 30 * time.Second,
			Jar:     jar,
		},
	}
}

// WithBasicAuth wires HTTP Basic Auth onto every outgoing request. Used when
// the deploy is sitting behind the temporary `SITE_PASSWORD` gate. Empty
// user OR password is a no-op.
func (c *Client) WithBasicAuth(user, pass string) *Client {
	if user == "" || pass == "" {
		return c
	}
	base := c.http.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	c.http.Transport = &basicAuthTransport{base: base, user: user, pass: pass}
	return c
}

type basicAuthTransport struct {
	base       http.RoundTripper
	user, pass string
}

func (t *basicAuthTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r2 := r.Clone(r.Context())
	r2.SetBasicAuth(t.user, t.pass)
	return t.base.RoundTrip(r2)
}

// WithDebug wraps the client's transport in a logging RoundTripper that
// emits a one-line summary of every request and response to stderr.
// A false enabled leaves the client unchanged.
func (c *Client) WithDebug(enabled, asJSON bool) *Client {
	if !enabled {
		return c
	}
	base := c.http.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	c.http.Transport = &debugTransport{base: base, asJSON: asJSON}
	return c
}

type debugTransport struct {
	base   http.RoundTripper
	asJSON bool
}

func (t *debugTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	start := time.Now()
	if t.asJSON {
		writeDebugJSON(map[string]any{
			"debug":  "request",
			"method": r.Method,
			"url":    r.URL.String(),
		})
	} else {
		fmt.Fprintf(os.Stderr, "[debug] → %s %s\n", r.Method, r.URL.String())
	}
	resp, err := t.base.RoundTrip(r)
	durMs := time.Since(start).Round(time.Millisecond).Milliseconds()
	if err != nil {
		if t.asJSON {
			writeDebugJSON(map[string]any{
				"debug":       "response",
				"error":       err.Error(),
				"duration_ms": durMs,
			})
		} else {
			fmt.Fprintf(os.Stderr, "[debug] ← ERR %v (%dms)\n", err, durMs)
		}
		return resp, err
	}
	if t.asJSON {
		writeDebugJSON(map[string]any{
			"debug":       "response",
			"status":      resp.StatusCode,
			"status_text": http.StatusText(resp.StatusCode),
			"duration_ms": durMs,
		})
	} else {
		fmt.Fprintf(os.Stderr, "[debug] ← %d %s (%dms)\n", resp.StatusCode, http.StatusText(resp.StatusCode), durMs)
	}
	return resp, nil
}

func writeDebugJSON(m map[string]any) {
	b, err := json.Marshal(m)
	if err != nil {
		return
	}
	fmt.Fprintln(os.Stderr, string(b))
}

// APIError is returned when the API responds with a non-2xx status.
type APIError struct {
	Status  int
	Message string
	Code    string
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("request failed (HTTP %d)", e.Status)
	}
	return e.Message
}

// IsUnauthorized reports whether err is an APIError with status 401.
func IsUnauthorized(err error) bool {
	var ae *APIError
	return errors.As(err, &ae) && ae.Status == http.StatusUnauthorized
}

// authMode controls how a request is authenticated.
type authMode int

const (
	authKey     authMode = iota // X-API-Key header (default)
	authSession                 // cookie only; no X-API-Key
	authNone                    // neither
)

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	return c.doWithAuth(ctx, method, path, body, out, authKey)
}

func (c *Client) doWithAuth(ctx context.Context, method, path string, body, out any, mode authMode) error {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if mode == authKey && c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		var apiErr struct {
			Error string `json:"error"`
			Code  string `json:"code"`
		}
		_ = json.Unmarshal(respBody, &apiErr)
		return &APIError{
			Status:  resp.StatusCode,
			Message: apiErr.Error,
			Code:    apiErr.Code,
		}
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
