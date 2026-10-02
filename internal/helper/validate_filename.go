package helper

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	ErrEmptyFilename       = errors.New("filename cannot be empty")
	ErrFilenameTooLong     = errors.New("filename exceeds maximum length of 255 characters")
	ErrPathTraversal       = errors.New("filename contains illegal path traversal characters")
	ErrInvalidFilenameChar = errors.New("filename contains invalid characters")
)

var validFilenameRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-. ]+$`)

// ValidateFilename checks whether a given filename is safe and valid.
func ValidateFilename(filename string) error {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return ErrEmptyFilename
	}

	if len(filename) > 255 {
		return ErrFilenameTooLong
	}

	// Prevent path traversal
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return ErrPathTraversal
	}

	// Verify against basename
	if filepath.Base(filename) != filename {
		return ErrPathTraversal
	}

	// Ensure no forbidden characters
	if !validFilenameRegex.MatchString(filename) {
		return ErrInvalidFilenameChar
	}

	return nil
}

