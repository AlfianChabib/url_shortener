package utils

import (
	"errors"
	"strings"
)

const (
	base62Chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	base        = uint64(len(base62Chars))
)

var ErrInvalidBase62Char = errors.New("invalid character in base62 string")

// EncodeBase62 converts a uint64 integer into a Base62 string.
func EncodeBase62(num uint64) string {
	if num == 0 {
		return string(base62Chars[0])
	}

	var sb strings.Builder
	for num > 0 {
		rem := num % base
		sb.WriteByte(base62Chars[rem])
		num /= base
	}

	// Reverse the bytes to get proper big-endian string representation
	runes := []rune(sb.String())
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}

	return string(runes)
}

// DecodeBase62 converts a Base62 string back to a uint64 integer.
func DecodeBase62(str string) (uint64, error) {
	var num uint64
	for i := 0; i < len(str); i++ {
		idx := strings.IndexByte(base62Chars, str[i])
		if idx == -1 {
			return 0, ErrInvalidBase62Char
		}
		num = num*base + uint64(idx)
	}
	return num, nil
}

