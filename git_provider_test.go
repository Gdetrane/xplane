package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseGitURL(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		expectedHost string
		expectedOwner string
		expectedRepo string
		expectError  bool
	}{
		{"github https", "https://github.com/user/repo.git", "github.com", "user", "repo", false},
		{"github https no .git", "https://github.com/user/repo", "github.com", "user", "repo", false},
		{"github ssh", "git@github.com:user/repo.git", "github.com", "user", "repo", false},
		{"github ssh no .git", "git@github.com:user/repo", "github.com", "user", "repo", false},
		{"gitlab https", "https://gitlab.com/group/project.git", "gitlab.com", "group", "project", false},
		{"self-hosted gitlab", "https://gitlab.example.com/team/project.git", "gitlab.example.com", "team", "project", false},
		{"ssh with port", "ssh://git@gitlab.example.com:2222/user/repo.git", "gitlab.example.com", "user", "repo", false},
		{"https with port", "https://gitlab.example.com:8080/user/repo.git", "gitlab.example.com", "user", "repo", false},
		{"invalid url", "not-a-url", "", "", "", true},
		{"incomplete ssh", "git@github.com", "", "", "", true},
		{"empty string", "", "", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, owner, repo, err := parseGitURL(tt.url)
			
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedHost, host)
				assert.Equal(t, tt.expectedOwner, owner)
				assert.Equal(t, tt.expectedRepo, repo)
			}
		})
	}
}