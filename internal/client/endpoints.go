package client

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

// ── Auth (session-cookie) ─────────────────────────────────────────────────
//
// Only used during `notifypulse login` to bootstrap an API key: we POST
// email+password, the server sets an `np_session` cookie in the jar, then we
// call /api/keys (also session-authed) to mint a long-lived CLI key, and
// finally /api/auth/logout to clean up. All day-to-day traffic then goes
// through /v1/* with the key.

// Login establishes a session cookie against /api/auth/login.
func (c *Client) Login(ctx context.Context, email, password string) (*User, error) {
	var out struct {
		User User `json:"user"`
	}
	req := map[string]string{"email": email, "password": password}
	if err := c.doWithAuth(ctx, "POST", "/api/auth/login", req, &out, authNone); err != nil {
		return nil, err
	}
	return &out.User, nil
}

// Signup creates a new user account and establishes a session cookie.
// Returns 403 if signups are disabled server-side.
func (c *Client) Signup(ctx context.Context, email, password, name string) (*User, error) {
	var out struct {
		User User `json:"user"`
	}
	req := map[string]string{"email": email, "password": password, "name": name}
	if err := c.doWithAuth(ctx, "POST", "/api/auth/signup", req, &out, authNone); err != nil {
		return nil, err
	}
	return &out.User, nil
}

// LogoutSession clears the session cookie server-side after key bootstrap.
func (c *Client) LogoutSession(ctx context.Context) error {
	return c.doWithAuth(ctx, "POST", "/api/auth/logout", nil, nil, authSession)
}

// Me returns the currently logged-in user (session-authed).
func (c *Client) Me(ctx context.Context) (*User, error) {
	var out struct {
		User User `json:"user"`
	}
	if err := c.doWithAuth(ctx, "GET", "/api/auth/me", nil, &out, authSession); err != nil {
		return nil, err
	}
	return &out.User, nil
}

// ── API keys (session-authed /api/keys) ───────────────────────────────────

// CreateAPIKeySession mints a new API key using the active session cookie.
// Called once by `notifypulse login` to bootstrap a CLI-owned key.
func (c *Client) CreateAPIKeySession(ctx context.Context, name string) (*CreatedAPIKey, error) {
	var out CreatedAPIKey
	req := map[string]string{"name": name}
	if err := c.doWithAuth(ctx, "POST", "/api/keys", req, &out, authSession); err != nil {
		return nil, err
	}
	return &out, nil
}

// ── API keys (key-authed is not supported server-side today) ──────────────
//
// The server only exposes /api/keys under session auth. That means the CLI
// can list/create/revoke keys only while a session is active — we reuse the
// login flow for each key-management call so scripts still work with just
// the stored credentials (email lookup via /api/auth/me on the session).
//
// For now the CLI's `keys` subcommand prompts for the password at use time
// (see cmd/keys.go) and runs an ephemeral session to hit /api/keys.

// ListAPIKeysSession lists keys through the active session cookie.
func (c *Client) ListAPIKeysSession(ctx context.Context) ([]APIKey, error) {
	var out struct {
		Keys []APIKey `json:"keys"`
	}
	if err := c.doWithAuth(ctx, "GET", "/api/keys", nil, &out, authSession); err != nil {
		return nil, err
	}
	return out.Keys, nil
}

// RevokeAPIKeySession revokes a key through the active session cookie.
func (c *Client) RevokeAPIKeySession(ctx context.Context, id string) error {
	return c.doWithAuth(ctx, "DELETE", "/api/keys/"+url.PathEscape(id), nil, nil, authSession)
}

// ── Dashboard (session-authed) ────────────────────────────────────────────

// DashboardSession fetches the overview stats. This endpoint is only
// exposed under /api/* (session auth); /v1 does not mirror it.
func (c *Client) DashboardSession(ctx context.Context) (*DashboardStats, error) {
	var out DashboardStats
	if err := c.doWithAuth(ctx, "GET", "/api/dashboard", nil, &out, authSession); err != nil {
		return nil, err
	}
	return &out, nil
}

// ── Destinations (/v1, API-key-authed) ────────────────────────────────────

type destinationEnvelope struct {
	Destinations []Destination `json:"destinations"`
}

func (c *Client) ListDestinations(ctx context.Context) ([]Destination, error) {
	var out destinationEnvelope
	if err := c.do(ctx, "GET", "/v1/destinations", nil, &out); err != nil {
		return nil, err
	}
	return out.Destinations, nil
}

func (c *Client) GetDestination(ctx context.Context, id string) (*Destination, error) {
	var out Destination
	if err := c.do(ctx, "GET", "/v1/destinations/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateDestinationRequest mirrors the server's POST /v1/destinations body.
type CreateDestinationRequest struct {
	Name    string          `json:"name"`
	Channel string          `json:"channel"`
	Config  json.RawMessage `json:"config"`
}

func (c *Client) CreateDestination(ctx context.Context, req CreateDestinationRequest) (*Destination, error) {
	var out Destination
	if err := c.do(ctx, "POST", "/v1/destinations", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteDestination(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/v1/destinations/"+url.PathEscape(id), nil, nil)
}

// ── Recipients (/v1, API-key-authed) ──────────────────────────────────────

type recipientEnvelope struct {
	Recipients []Recipient `json:"recipients"`
}

func (c *Client) ListRecipients(ctx context.Context) ([]Recipient, error) {
	var out recipientEnvelope
	if err := c.do(ctx, "GET", "/v1/recipients", nil, &out); err != nil {
		return nil, err
	}
	return out.Recipients, nil
}

func (c *Client) GetRecipient(ctx context.Context, name string) (*Recipient, error) {
	var out Recipient
	if err := c.do(ctx, "GET", "/v1/recipients/"+url.PathEscape(name), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateRecipientRequest mirrors the server's POST /v1/recipients body.
type CreateRecipientRequest struct {
	Name         string   `json:"name"`
	Destinations []string `json:"destinations,omitempty"`
}

func (c *Client) CreateRecipient(ctx context.Context, req CreateRecipientRequest) (*Recipient, error) {
	var out Recipient
	if err := c.do(ctx, "POST", "/v1/recipients", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteRecipient(ctx context.Context, name string) error {
	return c.do(ctx, "DELETE", "/v1/recipients/"+url.PathEscape(name), nil, nil)
}

// BindDestinations adds destinations to a recipient.
func (c *Client) BindDestinations(ctx context.Context, name string, destinations []string) ([]Destination, error) {
	var out struct {
		Destinations []Destination `json:"destinations"`
	}
	req := map[string][]string{"destinations": destinations}
	if err := c.do(ctx, "POST", "/v1/recipients/"+url.PathEscape(name)+"/destinations", req, &out); err != nil {
		return nil, err
	}
	return out.Destinations, nil
}

// UnbindDestination removes one destination from a recipient.
func (c *Client) UnbindDestination(ctx context.Context, name, destName string) error {
	return c.do(ctx, "DELETE",
		"/v1/recipients/"+url.PathEscape(name)+"/destinations/"+url.PathEscape(destName),
		nil, nil)
}

// ── Notify (/v1/notify) ───────────────────────────────────────────────────

// NotifyRequest is the body of POST /v1/notify.
type NotifyRequest struct {
	To           string   `json:"to,omitempty"`
	Destinations []string `json:"destinations,omitempty"`
	Title        string   `json:"title"`
	Body         string   `json:"body,omitempty"`
	Severity     string   `json:"severity,omitempty"`
	Link         string   `json:"link,omitempty"`
	DedupKey     string   `json:"dedup_key,omitempty"`
}

// NotifyResponse is the result of POST /v1/notify.
type NotifyResponse struct {
	ID         string           `json:"id"`
	Status     string           `json:"status"`
	Deliveries []DeliveryResult `json:"deliveries"`
	Deduped    bool             `json:"deduped,omitempty"`
}

func (c *Client) Notify(ctx context.Context, req NotifyRequest) (*NotifyResponse, error) {
	var out NotifyResponse
	if err := c.do(ctx, "POST", "/v1/notify", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ── Notifications (/v1/notifications, API-key-authed) ────────────────────

type notificationsEnvelope struct {
	Notifications []Notification `json:"notifications"`
}

// ListNotifications returns recent notifications (newest first).
func (c *Client) ListNotifications(ctx context.Context, limit int) ([]Notification, error) {
	path := "/v1/notifications"
	if limit > 0 {
		path += "?limit=" + strconv.Itoa(limit)
	}
	var out notificationsEnvelope
	if err := c.do(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return out.Notifications, nil
}

// GetNotification returns a single notification with its delivery rows.
func (c *Client) GetNotification(ctx context.Context, id string) (*Notification, error) {
	var out Notification
	if err := c.do(ctx, "GET", "/v1/notifications/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
