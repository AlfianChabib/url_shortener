package web

// RegisterRequest defines the payload for user registration.
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email,max=255"`
	Username string `json:"username" validate:"required,username_policy"`
	Password string `json:"password" validate:"required,password_policy"`
}

// LoginRequest defines the payload for user login (supports email or username as identifier).
type LoginRequest struct {
	Identifier string `json:"identifier" validate:"required,min=3,max=255"`
	Password   string `json:"password" validate:"required,min=1"`
}
