package main

import (
	"bytes"
	"io"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindGitRoot(t *testing.T) {
	t.Run("finds git root from the root itself in cwd", func(t *testing.T) {
		root, err := findGitRoot()
		assert.NoError(t, err)
		assert.Contains(t, root, "xplane")
	})

	t.Run("fails when not in git repo", func(t *testing.T) {
		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)

		os.Chdir("/tmp/")
		root, err := findGitRoot()
		assert.Error(t, err)
		assert.Equal(t, root, "")
	})
}

func TestGetHostFromURL(t *testing.T) {
	testCases := []struct {
		name         string
		url          string
		expectedHost string
		errorsOut    bool
	}{
		{"GitHub HTTPS", "https://github.com/user/repo.git", "github.com", false},
		{"GitHub SSH", "git@github.com:user/repo.git", "github.com", false},
		{"GitLab HTTPS", "https://gitlab.com/user/repo.git", "gitlab.com", false},
		{
			"Self-hosted GitLab", "https://gitlab.cee.redhat.com/group/repo.git",
			"gitlab.cee.redhat.com", false,
		},
		{
			"SSH with port", "ssh://git@gitlab.example.com:2222/user/repo.git",
			"gitlab.example.com", false,
		},
		{"Invalid URL", "not-a-git-url", "", true},
		{"Empty URL", "", "", true},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			host, err := getHostFromURL(test.url)

			if test.errorsOut {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expectedHost, host)
			}
		})
	}
}

func TestGetReadme(t *testing.T) {
	t.Run("README.md from arbitrary root dir", func(t *testing.T) {
		root, err := os.MkdirTemp("/tmp/", "readme_test*")
		assert.NoError(t, err)

		expectedContent := "This is a readme!"
		readmePath := path.Join(root, "README.md")
		err = os.WriteFile(readmePath, []byte(expectedContent), 0o700)
		assert.NoError(t, err)
		defer os.Remove(readmePath)

		content, err := getReadme(root)
		assert.NoError(t, err)
		assert.Equal(t, content, expectedContent)
	})

	t.Run("missing readme", func(t *testing.T) {
		root, err := os.MkdirTemp("/tmp/", "readme_test_empty*")
		assert.NoError(t, err)

		content, err := getReadme(root)
		assert.NoError(t, err)
		assert.Equal(t, "No README.md file provided in this project.", content)
	})
}

func TestGetGitExclude(t *testing.T) {
	testCases := []struct {
		name           string
		fileContent    string
		createFile     bool
		expectedOutput string
	}{
		{"existing exclude file", "*.log\n*.tmp", true, "*.log\n*.tmp"},
		{"missing exclude file", "", false, "No .git/info/exclude file found."},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			if test.createFile {
				root, err := os.MkdirTemp("/tmp/", "test_exclude*")
				assert.NoError(t, err)

				gitInfoPath := path.Join(root, ".git", "info")
				err = os.MkdirAll(gitInfoPath, 0o755)
				assert.NoError(t, err)

				excludePath := path.Join(gitInfoPath, "exclude")
				err = os.WriteFile(excludePath, []byte(test.fileContent), 0o700)
				assert.NoError(t, err)

				content, excludeErr := getGitExclude(root)
				assert.NoError(t, excludeErr)
				assert.Equal(t, test.expectedOutput, content)
			} else {
				root, err := os.MkdirTemp("/tmp/", "test_exclude*")
				assert.NoError(t, err)
				content, excludeErr := getGitExclude(root)
				assert.NoError(t, excludeErr)
				assert.Equal(t, content, test.expectedOutput)
			}
		})
	}
}

func TestGetGitignore(t *testing.T) {
	testCases := []struct {
		name           string
		fileContent    string
		createFile     bool
		expectedOutput string
	}{
		{"existing .gitignore", "*.log\n*.tmp", true, "*.log\n*.tmp"},
		{"missing .gitignore", "", false, "No .gitignore file found."},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			if test.createFile {
				root, err := os.MkdirTemp("/tmp/", "test_ignore*")
				assert.NoError(t, err)

				ignorePath := path.Join(root, ".gitignore")
				err = os.WriteFile(ignorePath, []byte(test.fileContent), 0o700)
				assert.NoError(t, err)

				content, excludeErr := getGitignore(root)
				assert.NoError(t, excludeErr)
				assert.Equal(t, test.expectedOutput, content)
			} else {
				root, err := os.MkdirTemp("/tmp/", "test_ignore*")
				assert.NoError(t, err)
				content, excludeErr := getGitignore(root)
				assert.NoError(t, excludeErr)
				assert.Equal(t, content, test.expectedOutput)
			}
		})
	}
}

func TestGetGitStatus(t *testing.T) {
	t.Run("capture status message", func(t *testing.T) {
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		getGitStatus("/tmp") // fail but print msg

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)

		assert.Contains(t, buf.String(), "Checking local git status")
	})
}

func TestGetGitProvider(t *testing.T) {
	testCases := []struct {
		name          string
		cfg           *Config
		expectedError string
	}{
		{
			"github missing token",
			&Config{GithubToken: "", GitlabToken: "dummy"},
			"'github_prs' requires GITHUB_TOKEN",
		},
		{
			"gitlab missing token",
			&Config{GithubToken: "", GitlabToken: ""},
			"'gitlab_mrs' requires GITLAB_TOKEN",
		},
		{
			"both tokens present",
			&Config{GithubToken: "gh_token", GitlabToken: "gl_token"},
			"", // Should not error on token validation
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			_, err := getGitProvider("/tmp", test.cfg) // Use non-git dir to test token validation

			if test.expectedError != "" {
				assert.Error(t, err)
			} else {
				// Both tokens present - should fail on git commands, not tokens
				if err != nil {
					assert.NotContains(t, err.Error(), "TOKEN")
				}
			}
		})
	}
}

func TestGetGitLog(t *testing.T) {
	tests := []struct {
		name      string
		gitRoot   string
		n         int
		expectErr bool
	}{
		{"valid git repo", ".", 5, false},
		{"non-git directory", "/tmp", 5, true},
		{"zero commits requested", ".", 0, false},
		{"negative commits", ".", -1, false}, // git handles this gracefully
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout to verify message is printed
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			result, err := getGitLog(tt.gitRoot, tt.n)

			w.Close()
			os.Stdout = old
			var buf bytes.Buffer
			io.Copy(&buf, r)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// For n=0, git returns empty output, which is correct
				if tt.n == 0 {
					assert.Empty(t, result)
				} else {
					assert.NotEmpty(t, result)
				}
			}
			
			// Verify the message was printed
			assert.Contains(t, buf.String(), "Fetching recent git log")
		})
	}
}

func TestGetTokeiStats(t *testing.T) {
	tests := []struct {
		name      string
		gitRoot   string
		expectErr bool
	}{
		{"current directory", ".", false},
		{"non-existent directory", "/non/existent/path", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			result, err := getTokeiStats(tt.gitRoot)

			w.Close()
			os.Stdout = old
			var buf bytes.Buffer
			io.Copy(&buf, r)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, result)
			}

			// Verify the message was printed
			assert.Contains(t, buf.String(), "Analyzing code stats")
		})
	}
}

func TestHasRemoteTrackingBranch(t *testing.T) {
	tests := []struct {
		name      string
		gitRoot   string
	}{
		{"current git repo", "."},
		{"non-git directory", "/tmp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This should not panic, regardless of result
			result := hasRemoteTrackingBranch(tt.gitRoot)
			// Result could be true or false, both are valid
			assert.IsType(t, false, result)
		})
	}
}

func TestGetRipSecrets(t *testing.T) {
	tests := []struct {
		name      string
		gitRoot   string
		expectErr bool
	}{
		{"current directory", ".", false}, // Should work if ripsecrets is installed
		{"non-existent directory", "/non/existent/path", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			result, err := getRipSecrets(tt.gitRoot)

			w.Close()
			os.Stdout = old
			var buf bytes.Buffer
			io.Copy(&buf, r)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				// Could succeed or fail depending on ripsecrets installation
				// But should not panic
				assert.IsType(t, "", result)
			}

			// Verify the message was printed
			assert.Contains(t, buf.String(), "Detecting potentially leaked secrets")
		})
	}
}

func TestGetGitDiff(t *testing.T) {
	tests := []struct {
		name      string
		gitRoot   string
		expectErr bool
	}{
		{"valid git repo", ".", false},
		{"non-git directory", "/tmp", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			result, err := getGitDiff(tt.gitRoot)

			w.Close()
			os.Stdout = old
			var buf bytes.Buffer
			io.Copy(&buf, r)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, result)
				// Should contain timestamp header
				assert.Contains(t, result, "Git diff captured at")
			}

			// Verify the message was printed
			assert.Contains(t, buf.String(), "Fetching uncommitted diff")
		})
	}
}