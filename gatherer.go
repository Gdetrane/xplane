package main

import "strings"

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

	owner, repo, err := parseGitURL(url)
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

	owner, repo, err := parseGitURL(url)
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
	if !hasRemoteTrackingBranch(cg.gitRoot) {
		output := "Local branch has not been pushed to the remote."
		return output, nil
	}

	if err := cg.initProvider(); err != nil {
		return "", nil
	}

	url, err := findPrimaryRemoteRepoURL(cg.gitRoot)
	if err != nil {
		return "", nil
	}

	owner, repo, err := parseGitURL(url)
	if err != nil {
		return "", err
	}

	localBranch, err := runCommand(cg.gitRoot, "git", "branch", "--show-current")
	if err != nil {
		return "", err
	}
	localBranch = strings.TrimSpace(localBranch)
	branchComparison, err := cg.gitProvider.CompareBranchWithDefault(owner, repo, localBranch)
	if err != nil {
		return "", err
	}

	return branchComparison.Format(), nil
}
