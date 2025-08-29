package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderMarkdown(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{"simple text", "Hello world", false},
		{"markdown formatting", "# Header\n**bold text**", false},
		{"empty string", "", false},
		{"multiline", "Line 1\nLine 2\nLine 3", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := renderMarkdown(tt.input)
			
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, result)
				assert.Contains(t, result, "██╗  ██╗██████╗") // Header should be included
				// For markdown rendering, the input gets processed/styled
				// so we just check that we got meaningful output
				assert.True(t, len(result) > len(tt.input))
			}
		})
	}
}