package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// generic command runner
func runCommand(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("command '%s' failed: %s, stderr: %s", name, err, stderr.String())
	}
	return out.String(), nil
}

// finds the top-level directory of the current git repository
func findGitRoot() (string, error) {
	output, err := runCommand(".", "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

// looks for an 'upstream' remote first, falling back to 'origin', in order to target the appropriate main for a fork based workflow
func findPrimaryRemoteRepoURL(gitRoot string) (string, error) {
	upstreamURL, err := runCommand(gitRoot, "git", "remote", "get-url", "upstream")
	if err == nil {
		return strings.TrimSpace(upstreamURL), nil
	}

	originURL, err := runCommand(gitRoot, "git", "remote", "get-url", "origin")
	if err != nil {
		return "", fmt.Errorf("failed to retrieve URL for 'upstream' or 'origin' remotes: %v", err)
	}
	return strings.TrimSpace(originURL), nil
}

// checks if the current git branch is tracking a remote branch.
func hasRemoteTrackingBranch(gitRoot string) bool {
	// fails if there is no upstream branch configured
	_, err := runCommand(gitRoot, "git", "rev-parse", "--abbrev-ref", "@{u}")
	return err == nil
}

func getHostFromURL(url string) (string, error) {
	host, _, _, err := parseGitURL(url)
	if err != nil {
		return "", err
	}

	return host, nil
}

func getOriginOwner(gitRoot string) (string, error) {
	originURL, err := runCommand(gitRoot, "git", "remote", "get-url", "origin")
	if err != nil {
		return "", fmt.Errorf("failed to retrieve origin remote URL to determine origin owner: %w", err)
	}

	originURL = strings.TrimSpace(originURL)
	_, owner, _, err := parseGitURL(originURL)
	return owner, err
}

func getGitProvider(gitRoot string, cfg *Config) (GitProvider, error) {
	primaryRemote, err := findPrimaryRemoteRepoURL(gitRoot) // upstream prevails in fork based workflows
	if err != nil {
		return nil, fmt.Errorf("xplane: error retrieving git remote provider: %v", err)
	}
	hostURL, getHostErr := getHostFromURL(primaryRemote)
	if getHostErr != nil {
		return nil, getHostErr
	}
	hostURL = "https://" + strings.TrimSpace(hostURL)

	// I need it anyways
	originRemote, err := runCommand(gitRoot, "git", "remote", "get-url", "origin")
	if err != nil {
		return nil, err
	}
	originRemote = strings.TrimSpace(originRemote)

	if strings.Contains(primaryRemote, "github") {
		if cfg.GithubToken == "" {
			return nil, fmt.Errorf("special command 'github_prs' requires GITHUB_TOKEN to be set")
		}
		return NewGitHubProvider(cfg.GithubToken, originRemote, primaryRemote), nil
	}

	if strings.Contains(primaryRemote, "gitlab") {
		if cfg.GitlabToken == "" {
			return nil, fmt.Errorf("special command 'gitlab_mrs' requires GITLAB_TOKEN to be set")
		}
		return NewGitlabProvider(cfg.GitlabToken, hostURL, originRemote, primaryRemote)
	}
	return nil, fmt.Errorf("xplane: unsupported git provider")
}

// returns git status in a machine parsable format using the low level porcelain format
func getGitStatus(gitRoot string) (string, error) {
	fmt.Println(MsgCheckingGitStatus)
	return runCommand(gitRoot, "git", "status", "--porcelain")
}

// returns a concise log of the latest N commits
func getGitLog(gitRoot string, n int) (string, error) {
	fmt.Println(MsgFetchingGitLog)
	return runCommand(gitRoot, "git", "log", "--oneline", "--graph", "--decorate", "-n", strconv.Itoa(n))
}

// returns code statistics in json format
func getTokeiStats(gitRoot string) (string, error) {
	fmt.Println(MsgGetCodeStats)
	return runCommand(gitRoot, "tokei", "--output", "json")
}

// returns potential leaked secrets
func getRipSecrets(gitRoot string) (string, error) {
	fmt.Println(MsgGetLeakedSecrets)
	cmd := exec.Command("ripsecrets", gitRoot)
	cmd.Dir = gitRoot
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()

	if exitErr, ok := err.(*exec.ExitError); ok {
		// for ripsecrets, a code of 1 just means secrets have been found, so I shouldn't exit
		if exitErr.ExitCode() == 1 {
			return out.String(), nil
		}
	}

	if err == nil {
		return "No secrets leaked.", nil
	}

	return "", fmt.Errorf("command 'ripsecrets' failed: %s, stderr: %s", err, stderr.String())
}

// reads and returns README.md's content if present, or a placeholder string
func getReadme(gitRoot string) (string, error) {
	var output string
	readmeBytes, readmeErr := os.ReadFile(filepath.Join(gitRoot, "README.md"))
	if os.IsNotExist(readmeErr) {
		output = "No README.md file provided in this project."
	} else if readmeErr != nil {
		return "", readmeErr
	} else {
		output = string(readmeBytes)
	}
	return output, nil
}

// reads and returns .git/info/exclude content if present, or a placeholder string
func getGitExclude(gitRoot string) (string, error) {
	excludeBytes, err := os.ReadFile(filepath.Join(gitRoot, ".git", "info", "exclude"))
	if os.IsNotExist(err) {
		return "No .git/info/exclude file found.", nil
	} else if err != nil {
		return "", err
	}
	return string(excludeBytes), nil
}

// reads and returns .gitignore content if present, or a placeholder string
func getGitignore(gitRoot string) (string, error) {
	gitignoreBytes, err := os.ReadFile(filepath.Join(gitRoot, ".gitignore"))
	if os.IsNotExist(err) {
		return "No .gitignore file found.", nil
	} else if err != nil {
		return "", err
	}
	return string(gitignoreBytes), nil
}

// returns git diff output showing latest changes
func getGitDiff(gitRoot string) (string, error) {
	fmt.Println(MsgFetchingGitDiff)
	diff, err := runCommand(gitRoot, "git", "diff")
	if err != nil {
		return "", err
	}

	// Add timestamp and explanatory context to help LLMs understand
	// that this shows uncommitted changes (static until committed)
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	header := fmt.Sprintf("Git diff captured at %s - Shows uncommitted changes (remains static until committed):\n\n", timestamp)

	if diff == "" {
		return header + "No uncommitted changes found.", nil
	}

	return header + diff, nil
}
