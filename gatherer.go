package main

import (
	"fmt"
	"strings"
)

type ContextGatherer struct {
	gitRoot     string
	cfg         *Config
	gitProvider GitProvider
}

func NewContextGatherer(gitRoot string, cfg *Config) *ContextGatherer {
	return &ContextGatherer{gitRoot: gitRoot, cfg: cfg}
}

func (cg *ContextGatherer) initProvider() error {
	if cg.gitProvider == nil {
		provider, err := getGitProvider(cg.gitRoot, cg.cfg)
		if err != nil {
			return err
		}
		cg.gitProvider = provider
	}
	return nil
}

func (cg *ContextGatherer) getOpenPRS() (string, error) {
	if err := cg.initProvider(); err != nil {
		return "", err
	}

	url, err := findPrimaryRemoteRepoURL(cg.gitRoot)
	if err != nil {
		return "", err
	}

	_, owner, repo, err := parseGitURL(url)
	if err != nil {
		return "", err
	}

	openPRS, err := cg.gitProvider.GetOpenPullRequests(owner, repo)
	if err != nil {
		return "", err
	}

	if len(openPRS) == 0 {
		return "No open pull/merge requests found.", nil
	}

	var builder strings.Builder
	for i, pr := range openPRS {
		builder.WriteString(pr.Format())
		if i < len(openPRS)-1 {
			builder.WriteString("\n---\n")
		}
	}
	output := builder.String()
	return output, nil
}

func (cg *ContextGatherer) getLatestRelease() (string, error) {
	if err := cg.initProvider(); err != nil {
		return "", nil
	}

	url, err := findPrimaryRemoteRepoURL(cg.gitRoot)
	if err != nil {
		return "", nil
	}

	_, owner, repo, err := parseGitURL(url)
	if err != nil {
		return "", err
	}

	release, err := cg.gitProvider.GetLatestRelease(owner, repo)
	if err != nil {
		return "", err
	}

	return release.Format(), nil
}

func (cg *ContextGatherer) getGitBranchStatus() (string, error) {
	// checking that the local branch has remote tracking first
	// this is not enough if a branch has been pushed but then removed from the remote
	// e.g. a branch could be autoremoved on the remote after a Merge and git wouldn't know locally without a git fetch --prune
	if !hasRemoteTrackingBranch(cg.gitRoot) {
		output := "Local branch has not been pushed to the remote."
		return output, nil
	}

	if err := cg.initProvider(); err != nil {
		return "", nil
	}

	localBranch, err := runCommand(cg.gitRoot, "git", "branch", "--show-current")
	if err != nil {
		return "", err
	}
	localBranch = strings.TrimSpace(localBranch)

	// adding an extra check on the remote itself
	_, _, repoName, err := parseGitURL(cg.gitProvider.GetRemoteURL())
	if err != nil {
		return "", err
	}
	originOwner, err := getOriginOwner(cg.gitRoot)
	if err != nil {
		return "", err
	}

	existsOnFork, err := cg.gitProvider.BranchExistsOnRemoteOrigin(originOwner, repoName, localBranch)
	if err != nil {
		return "", err
	}

	if !existsOnFork {
		return fmt.Sprintf("Local branch '%s' has been deleted from the remote fork.", localBranch), nil
	}

	url, err := findPrimaryRemoteRepoURL(cg.gitRoot)
	if err != nil {
		return "", nil
	}

	_, owner, repo, err := parseGitURL(url)
	if err != nil {
		return "", err
	}

	branchComparison, err := cg.gitProvider.CompareBranchWithDefault(owner, repo, originOwner, localBranch)
	if err != nil {
		return "", err
	}

	return branchComparison.Format(), nil
}
