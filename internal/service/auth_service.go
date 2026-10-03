package service

import (
	"context"
	"fmt"
	"time"
	"url_shortener/internal/config"
	"url_shortener/internal/model/domain"
	"url_shortener/internal/model/web"
	"url_shortener/internal/repository"
	"url_shortener/pkg/errs"
	"url_shortener/pkg/utils"

	"github.com/google/uuid"
)

// AuthService defines business logic for user registration, authentication, and profile management.
type AuthService interface {
	Register(ctx context.Context, req *web.RegisterRequest) (*web.UserResponse, error)
	Login(ctx context.Context, req *web.LoginRequest) (*web.LoginResponse, error)
	GetProfile(ctx context.Context, userID uuid.UUID) (*web.UserResponse, error)
}

type authServiceImpl struct {
	cfg      *config.Config
	userRepo repository.UserRepository
}

// NewAuthService creates a new AuthService instance.
func NewAuthService(cfg *config.Config, userRepo repository.UserRepository) AuthService {
	return &authServiceImpl{
		cfg:      cfg,
		userRepo: userRepo,
	}
}

func (s *authServiceImpl) Register(ctx context.Context, req *web.RegisterRequest) (*web.UserResponse, error) {
	// 1. Check if email already registered
	existingEmail, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err == nil && existingEmail != nil {
		return nil, errs.NewConflictError("email already registered")
	}

	// 2. Check if username already taken
	existingUser, err := s.userRepo.FindByUsername(ctx, req.Username)
	if err == nil && existingUser != nil {
		return nil, errs.NewConflictError("username already taken")
	}

	// 3. Hash password
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// 4. Generate UUIDv7
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("failed to generate uuidv7: %w", err)
	}

	now := time.Now()
	user := &domain.User{
		ID:           id,
		Email:        req.Email,
		Username:     req.Username,
		PasswordHash: hashedPassword,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	return &web.UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Username:  user.Username,
		CreatedAt: user.CreatedAt,
	}, nil
}

func (s *authServiceImpl) Login(ctx context.Context, req *web.LoginRequest) (*web.LoginResponse, error) {
	// 1. Find user by email or username
	user, err := s.userRepo.FindByIdentifier(ctx, req.Identifier)
	if err != nil || user == nil || !user.IsActive {
		return nil, errs.NewUnauthorizedError("invalid email/username or password")
	}

	// 2. Verify password
	if !utils.CheckPassword(req.Password, user.PasswordHash) {
		return nil, errs.NewUnauthorizedError("invalid email/username or password")
	}

	// 3. Determine token TTL
	ttlHours := s.cfg.JWT.ExpiresInHour
	if ttlHours <= 0 {
		ttlHours = 24
	}
	ttl := time.Duration(ttlHours) * time.Hour

	// 4. Generate JWT access token
	token, err := utils.GenerateToken(user.ID, user.Email, user.Username, s.cfg.JWT.Secret, ttl)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	return &web.LoginResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int(ttl.Seconds()),
		User: web.UserResponse{
			ID:        user.ID,
			Email:     user.Email,
			Username:  user.Username,
			CreatedAt: user.CreatedAt,
		},
	}, nil
}

func (s *authServiceImpl) GetProfile(ctx context.Context, userID uuid.UUID) (*web.UserResponse, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil {
		return nil, errs.NewNotFoundError("user not found")
	}

	return &web.UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Username:  user.Username,
		CreatedAt: user.CreatedAt,
	}, nil
}
