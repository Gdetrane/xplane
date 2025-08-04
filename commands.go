package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

// extracts the host (e.g. gitlab.cee.redhat.com) from a URL
func getHostFromURL(url string) string {
	re := regexp.MustCompile(`@(.*?):`)
	matches := re.FindStringSubmatch(url)
	if len(matches) > 1 {
		return "https://" + matches[1] // Return with https scheme
	}
	return ""
}

// returns a struct that implements the GitProvider interface, for the supported remote git providers (github, gitlab, etc)
func getGitProvider(gitRoot string, cfg *Config) (GitProvider, error) {
	remote, err := findPrimaryRemoteRepoURL(gitRoot)
	hostURL := getHostFromURL(remote)
	hostURL = strings.TrimSpace(hostURL)
	if err != nil {
		return nil, fmt.Errorf("xplane: error retrieving git remote provider: %v", err)
	}

	if strings.Contains(remote, "github") {
		if cfg.GithubToken == "" {
			return nil, fmt.Errorf("special command 'github_prs' requires GITHUB_TOKEN to be set")
		}
		return NewGitHubProvider(cfg.GithubToken), nil
	}

	if strings.Contains(remote, "gitlab") {
		if cfg.GitlabToken == "" {
			return nil, fmt.Errorf("command 'gitlab_mrs' requires GITLAB_TOKEN to be set")
		}
		return NewGitlabProvider(cfg.GitlabToken, hostURL)
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
	}
	output = string(readmeBytes)
	return output, nil
}
