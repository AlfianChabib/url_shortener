package domain

import (
	"time"

	"github.com/google/uuid"
)

type Link struct {
	ID          int64      `json:"id"`
	ShortCode   string     `json:"short_code"`
	OriginalURL string     `json:"original_url"`
	UserID      *uuid.UUID `json:"user_id,omitempty"`
	IsActive    bool       `json:"is_active"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

type ClickEvent struct {
	ShortCode   string    `json:"short_code"`
	ClickedAt   time.Time `json:"clicked_at"`
	IPHash      string    `json:"ip_hash"`
	CountryCode string    `json:"country_code"`
	City        string    `json:"city"`
	DeviceType  string    `json:"device_type"`
	Browser     string    `json:"browser"`
	OS          string    `json:"os"`
	Referer     string    `json:"referer"`
}

