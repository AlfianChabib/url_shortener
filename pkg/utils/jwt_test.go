package utils_test

import (
	"testing"
	"time"
	"url_shortener/pkg/utils"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestJWTGenerationAndValidation(t *testing.T) {
	secret := "test-secret-key-32-bytes-long-here!"
	userID, _ := uuid.NewV7()
	email := "test@example.com"
	username := "testuser"

	// 1. Generate Token
	token, err := utils.GenerateToken(userID, email, username, secret, 1*time.Hour)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// 2. Validate Token
	claims, err := utils.ValidateToken(token, secret)
	assert.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, email, claims.Email)
	assert.Equal(t, username, claims.Username)

	// 3. Invalid Secret
	_, err = utils.ValidateToken(token, "wrong-secret-key-that-fails-validation")
	assert.Error(t, err)

	// 4. Expired Token
	expiredToken, err := utils.GenerateToken(userID, email, username, secret, -1*time.Minute)
	assert.NoError(t, err)
	_, err = utils.ValidateToken(expiredToken, secret)
	assert.Error(t, err)
}
