package utils_test

import (
	"testing"
	"url_shortener/pkg/utils"

	"github.com/stretchr/testify/assert"
)

func TestPasswordHashing(t *testing.T) {
	raw := "SecurePassword123!"

	hash, err := utils.HashPassword(raw)
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, raw, hash)

	// Valid password
	assert.True(t, utils.CheckPassword(raw, hash))

	// Invalid password
	assert.False(t, utils.CheckPassword("WrongPassword!", hash))
}
