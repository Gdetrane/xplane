package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const defaultCommands = "git_status,git_log,readme,git_exclude,gitignore,git_diff,github_prs,gitlab_mrs,release,git_branch_status,tokei,ripsecrets"

var specialCommandToBinMap = map[string]string{
	"git_status":        "git",
	"git_log":           "git",
	"git_exclude":       "",
	"gitignore":         "",
	"git_diff":          "git",
	"tokei":             "tokei",
	"ripsecrets":        "ripsecrets",
	"github_prs":        "",
	"gitlab_mrs":        "",
	"git_branch_status": "",
	"release":           "",
	"readme":            "",
}

type Config struct {
	Commands            []string
	GithubToken         string
	GitlabToken         string
	Provider            string
	APIKey              string
	Model               string
	OllamaServerAddress string
}

func ensureBinaryInstalled(bin string) error {
	_, err := exec.LookPath(bin)
	if err != nil {
		if execErr, ok := err.(*exec.Error); ok && execErr.Err == exec.ErrNotFound {
			return fmt.Errorf("binary '%s' not found in $PATH", bin)
		}
		// unexpected errs
		return err
	}

	return nil
}

func loadConfig() (*Config, error) {
	cfg := &Config{
		GithubToken:         os.Getenv("GITHUB_TOKEN"),
		GitlabToken:         os.Getenv("GITLAB_TOKEN"),
		Provider:            os.Getenv("XPLANE_PROVIDER"),
		APIKey:              os.Getenv("XPLANE_API_KEY"),
		Model:               os.Getenv("XPLANE_MODEL"),
		OllamaServerAddress: os.Getenv("OLLAMA_HOST"),
	}

	if cfg.Provider == "" {
		cfg.Provider = "gemini_cli"
	}

	if cfg.Model == "" && cfg.Provider == "gemini_cli" {
		cfg.Model = "gemini-2.5-pro"
	}

	if cfg.Provider == "claude_code" && cfg.Model == "" {
		cfg.Model = "claude-sonnet-4"
	}

	if cfg.Provider == "ollama" {
		if cfg.OllamaServerAddress == "" {
			fmt.Println("No 'OLLAMA_HOST' provided, defaulting to 'http://localhost:11434'...")
			cfg.OllamaServerAddress = "http://localhost:11434"
		}
		if cfg.Model == "" {
			fmt.Println("No 'XPLANE_MODEL' provided, defaulting to 'gemma3n'...")
		}
	}

	commandsStr := os.Getenv("XPLANE_COMMANDS")
	if commandsStr == "" {
		commandsStr = defaultCommands
	}
	listOfCommands := strings.Split(commandsStr, ",")
	hasBeenChecked := make(map[string]bool) // I'll avoid checking repeating pkgs more than once
	missingBinaries := make([]string, 0)

	for _, command := range listOfCommands {
		trimmedCommand := strings.TrimSpace(command)
		binaryToCheck, isSpecial := specialCommandToBinMap[trimmedCommand]
		if !isSpecial {
			binaryToCheck = trimmedCommand
		}

		if binaryToCheck != "" && !hasBeenChecked[binaryToCheck] {
			if err := ensureBinaryInstalled(binaryToCheck); err != nil {
				missingBinaries = append(missingBinaries, binaryToCheck)
				return nil, err
			}
			hasBeenChecked[binaryToCheck] = true
		}
	}

	if len(missingBinaries) > 0 {
		return nil, fmt.Errorf("missing required packages: '%v', please ensure they're installed and in your $PATH", missingBinaries)
	}
	cfg.Commands = listOfCommands

	return cfg, nil
}
