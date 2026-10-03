package web

import (
	"time"

	"github.com/google/uuid"
)

// UserResponse defines the public user profile payload.
type UserResponse struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

// LoginResponse defines the JWT token and user response upon successful login.
type LoginResponse struct {
	AccessToken string       `json:"access_token"`
	TokenType   string       `json:"token_type"`
	ExpiresIn   int          `json:"expires_in"`
	User        UserResponse `json:"user"`
}
