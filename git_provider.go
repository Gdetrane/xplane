package main

import (
	"context"
	"fmt"
	"regexp"

	"github.com/google/go-github/v74/github"
	// "gitlab.com/gitlab-org/api/client-go"
	"golang.org/x/oauth2"
)

var gitURLRegex = regexp.MustCompile(`(?:git@|https://)[\w.-]+(?::|/)([\w.-]+)/([\w.-]+?)(\.git)?$`)

func parseGitURL(url string) (owner string, repo string, err error) {
	matches := gitURLRegex.FindStringSubmatch(url)

	if len(matches) < 3 {
		return "", "", fmt.Errorf("could not parse owner and repo from url: %s", url)
	}

	owner = matches[1]
	repo = matches[2]

	return owner, repo, nil
}

type GitProvider interface {
	GetOpenPullRequests(owner, repo string) ([]PullRequest, error)
	GetLatestRelease(owner, repo string) (Release, error)
	CompareBranchWithDefault(owner, repo, localBranch string) (BranchComparison, error)
}

type GithubProvider struct {
	client *github.Client
}

func (g *GithubProvider) GetOpenPullRequests(owner, repo string) ([]PullRequest, error) {
	prs, _, err := g.client.PullRequests.List(context.Background(), owner, repo, nil)
	if err != nil {
		return nil, fmt.Errorf("xplane: error fetching PRs from Github upstream: %v", err)
	}

	var results []PullRequest
	for _, pr := range prs {
		results = append(results, PullRequest{
			Title:       pr.GetTitle(),
			Author:      pr.GetUser().GetLogin(),
			Description: pr.GetBody(),
			URL:         pr.GetHTMLURL(),
		})
	}
	return results, nil
}

func (g *GithubProvider) GetLatestRelease(owner, repo string) (Release, error) {
	release, _, err := g.client.Repositories.GetLatestRelease(context.Background(), owner, repo)
	if err != nil {
		if _, ok := err.(*github.ErrorResponse); ok && err.(*github.ErrorResponse).Response.StatusCode >= 400 {
			// no release would get a 4xx, but this is not an issue and can be handled gracefully with relevant context
			return Release{TagName: "No releases found"}, nil
		}
		return Release{}, fmt.Errorf("xplane: error fetching latest release from Github: %v", err)
	}

	return Release{
		TagName:     release.GetTagName(),
		Name:        release.GetName(),
		URL:         release.GetHTMLURL(),
		PublishedAt: release.GetPublishedAt().Format("Sat, Nov 4, 1995"),
	}, nil
}

func (g *GithubProvider) CompareBranchWithDefault(owner, repo, localBranch string) (BranchComparison, error) {
	// finding repo's default branch
	repoInfo, _, err := g.client.Repositories.Get(context.Background(), owner, repo)
	if err != nil {
		return BranchComparison{}, fmt.Errorf("xplane: could not get repo info for default branch: %v", err)
	}
	defaultBranch := repoInfo.GetDefaultBranch()

	// obv not comparing to itself
	if localBranch == defaultBranch {
		return BranchComparison{Status: "identical"}, nil
	}

  // comparing the HEAD of local branch and default branch
	comparison, _, err := g.client.Repositories.CompareCommits(context.Background(), owner, repo, defaultBranch, localBranch, nil)
	if err != nil {
		return BranchComparison{}, fmt.Errorf("xplane: could not compare branches: %v", err)
	}

	return BranchComparison{
		AheadBy: comparison.GetAheadBy(),
		BehindBy: comparison.GetBehindBy(),
		Status: comparison.GetStatus(),
	}, nil
}

type GitlabProvider struct {
	repo  string
	token string
}

func (g *GitlabProvider) GetOpenPullRequests(owner, repo string) ([]PullRequest, error) {
	return nil, nil
}

func NewGitHubProvider(token string) *GithubProvider {
	ctx := context.Background()
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tokenClient := oauth2.NewClient(ctx, tokenSource)

	return &GithubProvider{
		client: github.NewClient(tokenClient),
	}
}

type PullRequest struct {
	Title       string
	Author      string
	Description string
	URL         string
}

type Release struct {
	TagName     string
	Name        string
	URL         string
	PublishedAt string
}

type BranchComparison struct {
	AheadBy  int
	BehindBy int
	Status   string
}
