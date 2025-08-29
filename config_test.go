package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnsureBinaryInstalled(t *testing.T) {
	tests := []struct {
		name      string
		binary    string
		expectErr bool
	}{
		{"existing binary", "ls", false},
		{"existing binary", "git", false},
		{"non-existing binary", "nonexistentbinary12345", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ensureBinaryInstalled(tt.binary)
			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "not found")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}