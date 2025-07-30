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

	if len(matches) < 3 {
		return "", "", fmt.Errorf("could not parse owner and repo from url: %s", url)
	}

	return owner, repo, nil
}

type GitProvider interface {
	GetOpenPullRequests(owner, repo string) ([]PullRequest, error)
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
			Title: pr.GetTitle(),
			Author: pr.GetUser().GetLogin(),
			Description: pr.GetBody(),
			URL: pr.GetHTMLURL(),
		})
	}
	return results, nil
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
