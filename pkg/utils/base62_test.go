package utils_test

import (
	"testing"
	"url_shortener/pkg/utils"

	"github.com/stretchr/testify/assert"
)

func TestBase62EncodeDecode(t *testing.T) {
	testCases := []uint64{
		0,
		1,
		61,
		62,
		123456789,
		1790953308803,
		^uint64(0),
	}

	for _, val := range testCases {
		encoded := utils.EncodeBase62(val)
		assert.NotEmpty(t, encoded)

		decoded, err := utils.DecodeBase62(encoded)
		assert.NoError(t, err)
		assert.Equal(t, val, decoded)
	}
}

func TestBase62DecodeInvalid(t *testing.T) {
	_, err := utils.DecodeBase62("invalid@char!")
	assert.Error(t, err)
}

