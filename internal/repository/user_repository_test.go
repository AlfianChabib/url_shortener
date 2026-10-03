package repository_test

import (
	"context"
	"testing"
	"time"
	"url_shortener/internal/model/domain"
	"url_shortener/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestUserRepository(t *testing.T) {
	repo := repository.NewUserRepository(nil)
	ctx := context.Background()

	id, _ := uuid.NewV7()
	user := &domain.User{
		ID:           id,
		Email:        "johndoe@example.com",
		Username:     "johndoe",
		PasswordHash: "hashed_secret_password",
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// 1. Create User
	err := repo.Create(ctx, user)
	assert.NoError(t, err)

	// Duplicate email create should error
	dupEmail := &domain.User{
		ID:           uuid.Must(uuid.NewV7()),
		Email:        "johndoe@example.com",
		Username:     "different_user",
		PasswordHash: "pass",
	}
	assert.Error(t, repo.Create(ctx, dupEmail))

	// Duplicate username create should error
	dupUsername := &domain.User{
		ID:           uuid.Must(uuid.NewV7()),
		Email:        "different@example.com",
		Username:     "johndoe",
		PasswordHash: "pass",
	}
	assert.Error(t, repo.Create(ctx, dupUsername))

	// 2. FindByID
	foundByID, err := repo.FindByID(ctx, id)
	assert.NoError(t, err)
	assert.Equal(t, user.Email, foundByID.Email)

	// 3. FindByEmail
	foundByEmail, err := repo.FindByEmail(ctx, "JOHNDOE@example.com")
	assert.NoError(t, err)
	assert.Equal(t, user.ID, foundByEmail.ID)

	// 4. FindByUsername
	foundByUsername, err := repo.FindByUsername(ctx, "JOHNDOE")
	assert.NoError(t, err)
	assert.Equal(t, user.ID, foundByUsername.ID)

	// 5. FindByIdentifier (using email)
	foundByIdentifierEmail, err := repo.FindByIdentifier(ctx, "johndoe@example.com")
	assert.NoError(t, err)
	assert.Equal(t, user.ID, foundByIdentifierEmail.ID)

	// 6. FindByIdentifier (using username)
	foundByIdentifierUsername, err := repo.FindByIdentifier(ctx, "johndoe")
	assert.NoError(t, err)
	assert.Equal(t, user.ID, foundByIdentifierUsername.ID)

	// 7. Not found identifier
	_, err = repo.FindByIdentifier(ctx, "unknown_user")
	assert.Error(t, err)
}
