package web

// CreateLinkRequest defines the request body payload for creating a shortened URL.
type CreateLinkRequest struct {
	OriginalURL    string `json:"original_url" validate:"required,url,max=2048"`
	CustomAlias    string `json:"custom_alias,omitempty" validate:"omitempty,min=4,max=32"`
	ExpiresInHours int    `json:"expires_in_hours,omitempty" validate:"omitempty,min=1"`
}

