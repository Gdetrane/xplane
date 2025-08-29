package main

import (
	"bytes"
	"io"
	"os"
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

func TestLoadConfig(t *testing.T) {
	// Helper function to clear and set environment variables
	setEnvVars := func(envVars map[string]string) {
		// Clear all relevant env vars first
		envKeys := []string{
			"GITHUB_TOKEN", "GITLAB_TOKEN", "XPLANE_PROVIDER", "XPLANE_API_KEY",
			"XPLANE_MODEL", "OLLAMA_HOST", "USE_PROJECT_KNOWLEDGE", "XPLANE_COMMANDS",
		}
		for _, key := range envKeys {
			os.Unsetenv(key)
		}
		
		// Set the ones we want for this test
		for key, value := range envVars {
			os.Setenv(key, value)
		}
	}

	// Cleanup function to restore original env
	originalEnv := make(map[string]string)
	envKeys := []string{
		"GITHUB_TOKEN", "GITLAB_TOKEN", "XPLANE_PROVIDER", "XPLANE_API_KEY",
		"XPLANE_MODEL", "OLLAMA_HOST", "USE_PROJECT_KNOWLEDGE", "XPLANE_COMMANDS",
	}
	for _, key := range envKeys {
		originalEnv[key] = os.Getenv(key)
	}
	
	defer func() {
		for key, value := range originalEnv {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()

	tests := []struct {
		name           string
		envVars        map[string]string
		expectedConfig *Config
		expectError    bool
		expectPrint    bool
		printContains  []string
	}{
		{
			name:    "default configuration",
			envVars: map[string]string{},
			expectedConfig: &Config{
				Provider: "gemini_cli",
				Model:    "gemini-2.5-pro",
				UseProjectKnowledge: false,
			},
			expectError: false,
		},
		{
			name: "all environment variables set",
			envVars: map[string]string{
				"GITHUB_TOKEN":         "gh_token",
				"GITLAB_TOKEN":         "gl_token", 
				"XPLANE_PROVIDER":      "claude_code",
				"XPLANE_API_KEY":       "api_key",
				"XPLANE_MODEL":         "custom-model",
				"OLLAMA_HOST":          "http://custom:8080",
				"USE_PROJECT_KNOWLEDGE": "true",
				"XPLANE_COMMANDS":      "git_status,readme",
			},
			expectedConfig: &Config{
				GithubToken:         "gh_token",
				GitlabToken:         "gl_token",
				Provider:            "claude_code",
				APIKey:              "api_key",
				Model:               "custom-model",
				OllamaServerAddress: "http://custom:8080",
				UseProjectKnowledge: true,
			},
			expectError: false,
		},
		{
			name: "claude_code provider defaults",
			envVars: map[string]string{
				"XPLANE_PROVIDER": "claude_code",
			},
			expectedConfig: &Config{
				Provider: "claude_code",
				Model:    "claude-sonnet-4",
				UseProjectKnowledge: false,
			},
			expectError: false,
		},
		{
			name: "ollama provider with defaults",
			envVars: map[string]string{
				"XPLANE_PROVIDER": "ollama",
			},
			expectedConfig: &Config{
				Provider:            "ollama",
				OllamaServerAddress: "http://localhost:11434",
				UseProjectKnowledge: false,
			},
			expectError:   false,
			expectPrint:   true,
			printContains: []string{"No 'OLLAMA_HOST' provided", "No 'XPLANE_MODEL' provided"},
		},
		{
			name: "ollama provider with custom settings",
			envVars: map[string]string{
				"XPLANE_PROVIDER": "ollama",
				"OLLAMA_HOST":     "http://custom:11434",
				"XPLANE_MODEL":    "llama2",
			},
			expectedConfig: &Config{
				Provider:            "ollama",
				Model:               "llama2",
				OllamaServerAddress: "http://custom:11434",
				UseProjectKnowledge: false,
			},
			expectError: false,
		},
		{
			name: "USE_PROJECT_KNOWLEDGE variations",
			envVars: map[string]string{
				"USE_PROJECT_KNOWLEDGE": "false",
			},
			expectedConfig: &Config{
				Provider:            "gemini_cli",
				Model:               "gemini-2.5-pro",
				UseProjectKnowledge: false,
			},
			expectError: false,
		},
		{
			name: "custom commands",
			envVars: map[string]string{
				"XPLANE_COMMANDS": "git_status,readme,custom_command",
			},
			expectedConfig: &Config{
				Provider:            "gemini_cli",
				Model:               "gemini-2.5-pro",
				UseProjectKnowledge: false,
			},
			expectError: true, // Should fail because custom_command binary doesn't exist
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnvVars(tt.envVars)

			// Capture stdout if we expect prints
			var buf bytes.Buffer
			if tt.expectPrint {
				old := os.Stdout
				r, w, _ := os.Pipe()
				os.Stdout = w

				cfg, err := loadConfig()

				w.Close()
				os.Stdout = old
				io.Copy(&buf, r)

				// Check printed output
				output := buf.String()
				for _, expectedText := range tt.printContains {
					assert.Contains(t, output, expectedText)
				}

				if tt.expectError {
					assert.Error(t, err)
					return
				}
				assert.NoError(t, err)
				
				// Verify config values (not testing Commands since they're complex)
				assert.Equal(t, tt.expectedConfig.Provider, cfg.Provider)
				assert.Equal(t, tt.expectedConfig.Model, cfg.Model)
				assert.Equal(t, tt.expectedConfig.OllamaServerAddress, cfg.OllamaServerAddress)
				assert.Equal(t, tt.expectedConfig.UseProjectKnowledge, cfg.UseProjectKnowledge)
				if tt.expectedConfig.GithubToken != "" {
					assert.Equal(t, tt.expectedConfig.GithubToken, cfg.GithubToken)
				}
				if tt.expectedConfig.GitlabToken != "" {
					assert.Equal(t, tt.expectedConfig.GitlabToken, cfg.GitlabToken)
				}
				if tt.expectedConfig.APIKey != "" {
					assert.Equal(t, tt.expectedConfig.APIKey, cfg.APIKey)
				}
			} else {
				cfg, err := loadConfig()
				
				if tt.expectError {
					assert.Error(t, err)
					return
				}
				assert.NoError(t, err)
				
				// Verify config values
				assert.Equal(t, tt.expectedConfig.Provider, cfg.Provider)
				assert.Equal(t, tt.expectedConfig.Model, cfg.Model)
				assert.Equal(t, tt.expectedConfig.UseProjectKnowledge, cfg.UseProjectKnowledge)
				if tt.expectedConfig.GithubToken != "" {
					assert.Equal(t, tt.expectedConfig.GithubToken, cfg.GithubToken)
				}
				if tt.expectedConfig.GitlabToken != "" {
					assert.Equal(t, tt.expectedConfig.GitlabToken, cfg.GitlabToken)
				}
				if tt.expectedConfig.APIKey != "" {
					assert.Equal(t, tt.expectedConfig.APIKey, cfg.APIKey)
				}
			}
		})
	}
}