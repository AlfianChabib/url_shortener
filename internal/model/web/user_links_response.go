package web

import (
	"time"

	"github.com/google/uuid"
)

// UserLinkItem represents a single link item belonging to the authenticated user.
type UserLinkItem struct {
	ID          uuid.UUID  `json:"id"`
	ShortCode   string     `json:"short_code"`
	ShortURL    string     `json:"short_url"`
	OriginalURL string     `json:"original_url"`
	IsActive    bool       `json:"is_active"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// UserLinksResponse represents the paginated link list for the user dashboard.
type UserLinksResponse struct {
	Total int64          `json:"total"`
	Page  int            `json:"page"`
	Limit int            `json:"limit"`
	Links []UserLinkItem `json:"links"`
}
