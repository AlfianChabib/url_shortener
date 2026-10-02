package helper_test

import (
	"testing"
	"url_shortener/internal/helper"

	"github.com/stretchr/testify/assert"
)

func TestValidateFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantErr  bool
	}{
		{
			name:     "valid simple filename",
			filename: "document.pdf",
			wantErr:  false,
		},
		{
			name:     "valid filename with underscores and hyphens",
			filename: "my_report-2026.docx",
			wantErr:  false,
		},
		{
			name:     "empty filename",
			filename: "",
			wantErr:  true,
		},
		{
			name:     "path traversal with double dots",
			filename: "../secret.txt",
			wantErr:  true,
		},
		{
			name:     "path traversal with slash",
			filename: "folder/file.png",
			wantErr:  true,
		},
		{
			name:     "path traversal with backslash",
			filename: "folder\\file.png",
			wantErr:  true,
		},
		{
			name:     "special forbidden characters",
			filename: "file*name?.txt",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := helper.ValidateFilename(tt.filename)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

