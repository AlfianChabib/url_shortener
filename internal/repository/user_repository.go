package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"url_shortener/internal/model/domain"
	"url_shortener/pkg/errs"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserRepository defines data access methods for users.
type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByUsername(ctx context.Context, username string) (*domain.User, error)
	FindByIdentifier(ctx context.Context, identifier string) (*domain.User, error)
}

type userRepositoryImpl struct {
	db *gorm.DB

	// In-memory fallback stores for testing/offline mode
	mu       sync.RWMutex
	memUsers map[uuid.UUID]*domain.User
}

// NewUserRepository creates a new UserRepository instance.
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepositoryImpl{
		db:       db,
		memUsers: make(map[uuid.UUID]*domain.User),
	}
}

func (r *userRepositoryImpl) Create(ctx context.Context, user *domain.User) error {
	if r.db != nil {
		if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return errs.NewConflictError("email or username already exists", err)
			}
			return fmt.Errorf("failed to insert user: %w", err)
		}
		return nil
	}

	// In-memory fallback
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, u := range r.memUsers {
		if strings.EqualFold(u.Email, user.Email) {
			return errs.NewConflictError("email already registered")
		}
		if strings.EqualFold(u.Username, user.Username) {
			return errs.NewConflictError("username already taken")
		}
	}

	r.memUsers[user.ID] = user
	return nil
}

func (r *userRepositoryImpl) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	if r.db != nil {
		var user domain.User
		err := r.db.WithContext(ctx).
			Where("id = ? AND is_active = ?", id, true).
			First(&user).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errs.NewNotFoundError("user not found")
			}
			return nil, fmt.Errorf("failed to query user by id: %w", err)
		}
		return &user, nil
	}

	// In-memory fallback
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, exists := r.memUsers[id]
	if !exists || !user.IsActive {
		return nil, errs.NewNotFoundError("user not found")
	}

	return user, nil
}

func (r *userRepositoryImpl) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	if r.db != nil {
		var user domain.User
		err := r.db.WithContext(ctx).
			Where("LOWER(email) = LOWER(?) AND is_active = ?", email, true).
			First(&user).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errs.NewNotFoundError("user not found")
			}
			return nil, fmt.Errorf("failed to query user by email: %w", err)
		}
		return &user, nil
	}

	// In-memory fallback
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, u := range r.memUsers {
		if strings.EqualFold(u.Email, email) && u.IsActive {
			return u, nil
		}
	}

	return nil, errs.NewNotFoundError("user not found")
}

func (r *userRepositoryImpl) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	if r.db != nil {
		var user domain.User
		err := r.db.WithContext(ctx).
			Where("LOWER(username) = LOWER(?) AND is_active = ?", username, true).
			First(&user).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errs.NewNotFoundError("user not found")
			}
			return nil, fmt.Errorf("failed to query user by username: %w", err)
		}
		return &user, nil
	}

	// In-memory fallback
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, u := range r.memUsers {
		if strings.EqualFold(u.Username, username) && u.IsActive {
			return u, nil
		}
	}

	return nil, errs.NewNotFoundError("user not found")
}

func (r *userRepositoryImpl) FindByIdentifier(ctx context.Context, identifier string) (*domain.User, error) {
	if r.db != nil {
		var user domain.User
		err := r.db.WithContext(ctx).
			Where("(LOWER(email) = LOWER(?) OR LOWER(username) = LOWER(?)) AND is_active = ?", identifier, identifier, true).
			First(&user).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errs.NewNotFoundError("user not found")
			}
			return nil, fmt.Errorf("failed to query user by identifier: %w", err)
		}
		return &user, nil
	}

	// In-memory fallback
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, u := range r.memUsers {
		if (strings.EqualFold(u.Email, identifier) || strings.EqualFold(u.Username, identifier)) && u.IsActive {
			return u, nil
		}
	}

	return nil, errs.NewNotFoundError("user not found")
}
