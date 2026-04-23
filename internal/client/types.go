package client

import (
	"encoding/json"
	"time"
)

// User mirrors the server auth.User model.
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Destination is a bound channel for a single user.
type Destination struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Channel   string          `json:"channel"`
	Config    json.RawMessage `json:"config"`
	CreatedAt time.Time       `json:"created_at"`
}

// Recipient is a named fan-out group containing zero or more destinations.
type Recipient struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	CreatedAt    time.Time     `json:"created_at"`
	Destinations []Destination `json:"destinations,omitempty"`
}

// Notification is a send record — title/body plus fan-out metadata.
type Notification struct {
	ID         string        `json:"id"`
	Title      string        `json:"title"`
	Body       string        `json:"body,omitempty"`
	Severity   string        `json:"severity"`
	Link       string        `json:"link,omitempty"`
	DedupKey   string        `json:"dedup_key,omitempty"`
	CreatedAt  time.Time     `json:"created_at"`
	Deliveries []DeliveryRow `json:"deliveries,omitempty"`
}

// DeliveryRow is one per-destination outcome persisted in delivery_log.
type DeliveryRow struct {
	ID              string    `json:"id"`
	DestinationID   string    `json:"destination_id"`
	DestinationName string    `json:"destination_name"`
	Channel         string    `json:"channel"`
	Status          string    `json:"status"`
	Error           string    `json:"error,omitempty"`
	DeliveredAt     time.Time `json:"delivered_at"`
}

// DeliveryResult is one per-destination outcome returned inline from /notify.
type DeliveryResult struct {
	DestinationID   string `json:"destination_id"`
	DestinationName string `json:"destination_name"`
	Channel         string `json:"channel"`
	Status          string `json:"status"`
	Error           string `json:"error,omitempty"`
}

// APIKey is an API-key record without the raw secret.
type APIKey struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Name      string     `json:"name"`
	Prefix    string     `json:"prefix"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// CreatedAPIKey includes the raw secret — only returned at creation time.
type CreatedAPIKey struct {
	APIKey
	RawKey string `json:"raw_key"`
}

// DashboardStats is the overview snapshot returned by GET /api/dashboard.
type DashboardStats struct {
	Destinations        int `json:"destinations"`
	Recipients          int `json:"recipients"`
	APIKeys             int `json:"api_keys"`
	Notifications24h    int `json:"notifications_24h"`
	DeliveriesSent24h   int `json:"deliveries_sent_24h"`
	DeliveriesFailed24h int `json:"deliveries_failed_24h"`
}
