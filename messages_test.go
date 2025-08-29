package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildRemoteInfoMsg(t *testing.T) {
	tests := []struct {
		name         string
		providerName string
		commandName  string
		expected     string
	}{
		{"github release", "github", "release", "    - \uF09B     Fetching info from GitHub: Getting latest release..."},
		{"github prs", "github", "github_prs", "    - \uF09B     Fetching info from GitHub: Getting open PRs..."},
		{"github branch status", "github", "git_branch_status", "    - \uF09B     Fetching info from GitHub: Comparing current branch to upstream..."},
		{"gitlab release", "gitlab", "release", "    - \ue65c     Fetching info from GitLab: Getting latest release..."},
		{"gitlab mrs", "gitlab", "gitlab_mrs", "    - \ue65c     Fetching info from GitLab: Getting open MRs..."},
		{"gitlab branch status", "gitlab", "git_branch_status", "    - \ue65c     Fetching info from GitLab: Comparing current branch to upstream..."},
		{"unknown provider", "unknown", "release", "Unexpected command: release"},
		{"unknown command", "github", "unknown", "Unexpected git provider: github"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildRemoteInfoMsg(tt.providerName, tt.commandName)
			assert.Equal(t, tt.expected, result)
		})
	}
}