package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
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

// returns a struct that implements the GitProvider interface, for the supported remote git providers (github, gitlab, etc)
func getGitProvider(gitRoot string, cfg *Config) (GitProvider, error) {
	remote, err := findPrimaryRemoteRepoURL(gitRoot)
	if err != nil {
		return nil, fmt.Errorf("xplane: error retrieving git remote provider: %v", err)
	}

	if strings.Contains(remote, "github") {
		if cfg.GithubToken == "" {
			return nil, fmt.Errorf("special command 'github_prs' requires GITHUB_TOKEN to be set")
		}
		return NewGitHubProvider(cfg.GithubToken), nil
	}
	return nil, fmt.Errorf("xplane: unsupported git provider")
}

// returns git status in a machine parsable format using the low level porcelain format
func getGitStatus(gitRoot string) (string, error) {
	return runCommand(gitRoot, "git", "status", "--porcelain")
}

// returns a concise log of the latest N commits
func getGitLog(gitRoot string, n int) (string, error) {
	return runCommand(gitRoot, "git", "log", "--oneline", "--graph", "--decorate", "-n", strconv.Itoa(n))
}

// returns code statistics in json format
func getTokeiStats(gitRoot string) (string, error) {
	return runCommand(gitRoot, "tokei", "--output", "json")
}

// returns potential leaked secrets
func getRipSecrets(gitRoot string) (string, error) {
	return runCommand(gitRoot, "ripsecrets")
}
