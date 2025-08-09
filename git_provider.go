package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/go-github/v74/github"
	"gitlab.com/gitlab-org/api/client-go"
	"golang.org/x/oauth2"
)

var gitURLRegex = regexp.MustCompile(`(?:git@|https://)([\w.-]+)(?::|/)([\w.-]+)/([\w.-]+?)(\.git)?$`)

func parseGitURL(url string) (host string, owner string, repoName string, err error) {
	matches := gitURLRegex.FindStringSubmatch(url)

	if len(matches) < 4 {
		return "", "", "", fmt.Errorf("could not parse owner and repo from url: %s", url)
	}

	host = matches[1]
	owner = matches[2]
	repoName = matches[3]

	return host, owner, repoName, nil
}

type GitProvider interface {
	GetProviderName() string
	GetRemoteURL() string
	GetUpstreamURL() string
	BranchExistsOnRemoteOrigin(owner, repo, branchName string) (bool, error)
	GetOpenPullRequests(owner, repo string) ([]PullRequest, error)
	GetLatestRelease(owner, repo string) (Release, error)
	CompareBranchWithDefault(owner, repo, forkOwner, localBranch string) (BranchComparison, error)
}

type GithubProvider struct {
	client            *github.Client
	remoteOriginURL   string
	remoteUpstreamURL string
}

func (g *GithubProvider) GetProviderName() string {
	return "github"
}

func (g *GithubProvider) GetRemoteURL() string {
	return g.remoteOriginURL
}

func (g *GithubProvider) GetUpstreamURL() string {
	return g.remoteUpstreamURL
}

func (g *GithubProvider) BranchExistsOnRemoteOrigin(owner, repo, branchName string) (bool, error) {
	_, resp, err := g.client.Repositories.GetBranch(context.Background(), owner, repo, branchName, 1)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			// branch doesn't exist
			return false, nil
		}
		return false, err
	}
	return true, nil
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

func (g *GithubProvider) CompareBranchWithDefault(owner, repo, forkOwner, localBranch string) (BranchComparison, error) {
	// finding repo's default branch
	repoInfo, _, err := g.client.Repositories.Get(context.Background(), owner, repo)
	if err != nil {
		return BranchComparison{}, fmt.Errorf("xplane: could not get repo info for default branch: %v", err)
	}
	defaultBranch := repoInfo.GetDefaultBranch()

	// obv not comparing to itself
	if localBranch == defaultBranch && owner == forkOwner {
		return BranchComparison{Status: "identical"}, nil
	}

	// using format "owner:branch"
	baseRef := defaultBranch
	headRef := fmt.Sprintf("%s:%s", forkOwner, localBranch)

	// comparing the HEAD of local branch and default branch
	comparison, _, err := g.client.Repositories.CompareCommits(context.Background(), owner, repo, baseRef, headRef, nil)
	if err != nil {
		return BranchComparison{}, fmt.Errorf("xplane: could not compare branches: %v", err)
	}

	return BranchComparison{
		AheadBy:  comparison.GetAheadBy(),
		BehindBy: comparison.GetBehindBy(),
		Status:   comparison.GetStatus(),
	}, nil
}

type GitlabProvider struct {
	client            *gitlab.Client
	remoteOriginURL   string
	remoteUpstreamURL string
}

func (g *GitlabProvider) GetProviderName() string {
	return "gitlab"
}

func (g *GitlabProvider) GetRemoteURL() string {
	return g.remoteOriginURL
}

func (g *GitlabProvider) GetUpstreamURL() string {
	return g.remoteUpstreamURL
}

func (g *GitlabProvider) BranchExistsOnRemoteOrigin(owner, repo, branchName string) (bool, error) {
	projectID := fmt.Sprintf("%s/%s", owner, repo)
	_, resp, err := g.client.Branches.GetBranch(projectID, branchName)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			// branch doesn't exist
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (g *GitlabProvider) GetOpenPullRequests(owner, repo string) ([]PullRequest, error) {
	// gitlab's api is slightly different, owner and repo are bundled into a project id like "owner/repo"
	projectID := fmt.Sprintf("%s/%s", owner, repo)

	prState := "opened"
	// I'm unifying notation but technically gitlab calls them Merge Requests
	opts := &gitlab.ListProjectMergeRequestsOptions{
		State: &prState,
	}
	mrs, _, err := g.client.MergeRequests.ListProjectMergeRequests(projectID, opts)
	if err != nil {
		return nil, fmt.Errorf("xplane: error fetching MRs from Gitlab: %v", err)
	}

	var results []PullRequest

	for _, mr := range mrs {
		results = append(results, PullRequest{
			Title:       mr.Title,
			Author:      mr.Author.Username,
			Description: mr.Description,
			URL:         mr.WebURL,
		})
	}

	return results, nil
}

func (g *GitlabProvider) GetLatestRelease(owner, repo string) (Release, error) {
	projectID := fmt.Sprintf("%s/%s", owner, repo)

	opts := &gitlab.ListReleasesOptions{
		ListOptions: gitlab.ListOptions{
			Page:    1,
			PerPage: 1,
		},
	}

	releases, _, err := g.client.Releases.ListReleases(projectID, opts)
	if err != nil {
		return Release{}, fmt.Errorf("xplane: error fetching releases from Gitlab: %v", err)
	}
	if len(releases) == 0 {
		return Release{TagName: "No releases found"}, nil
	}

	latest := releases[0]
	return Release{
		TagName:     latest.TagName,
		Name:        latest.Name,
		URL:         latest.Links.Self,
		PublishedAt: latest.ReleasedAt.Format("Sat, Nov 4, 1995"),
	}, nil
}

// helper that simplifies fetching commits from paged gitlab content
func (g *GitlabProvider) getAllCommits(projectID, branchName string) ([]*gitlab.Commit, error) {
	opts := &gitlab.ListCommitsOptions{
		RefName: &branchName,
		ListOptions: gitlab.ListOptions{
			PerPage: 100, // max value allowed per page
			Page:    1,
		},
	}

	var allCommits []*gitlab.Commit

	for {
		commits, resp, err := g.client.Commits.ListCommits(projectID, opts)
		if err != nil {
			return nil, err
		}
		allCommits = append(allCommits, commits...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return allCommits, nil
}

func (g *GitlabProvider) CompareBranchWithDefault(owner, repo, forkOwner, localBranch string) (BranchComparison, error) {
	upstreamProjectID := fmt.Sprintf("%s/%s", owner, repo)
	forkProjectID := fmt.Sprintf("%s/%s", forkOwner, repo)

	project, _, err := g.client.Projects.GetProject(upstreamProjectID, nil)
	if err != nil {
		return BranchComparison{}, fmt.Errorf("xplane: could not get Gitlab repo info: %v", err)
	}
	defaultBranch := project.DefaultBranch

	if localBranch == defaultBranch && owner == forkOwner {
		return BranchComparison{Status: "identical"}, nil
	}

	// I need to implement cross-fork comparison logic manually
	upstreamCommits, err := g.getAllCommits(upstreamProjectID, defaultBranch)
	if err != nil {
		return BranchComparison{}, fmt.Errorf("xplane: could not list commits for upstream default branch: %w", err)
	}
	upstreamCommitMap := make(map[string]bool)
	for _, commit := range upstreamCommits {
		upstreamCommitMap[commit.ID] = true
	}

	forkCommits, err := g.getAllCommits(forkProjectID, localBranch)
	if err != nil {
		return BranchComparison{}, fmt.Errorf("xplane: could not list commits for remote origin branch: '%s': %w", localBranch, err)
	}

	// comparing fork and upstream until a merge base is found, any other commit is ahead and thus I can increase the count
	var mergeBaseSHA string
	aheadBy := 0
	for _, commit := range forkCommits {
		if upstreamCommitMap[commit.ID] {
			mergeBaseSHA = commit.ID
			break
		}
		aheadBy++
	}

	if mergeBaseSHA == "" {
		return BranchComparison{}, fmt.Errorf("could not find a common ancestor for the compared branches")
	}
	// now I can check how behind the current local branch is from the upstream -> increase until the merge base is reached
	behindBy := 0
	for _, commit := range upstreamCommits {
		if commit.ID == mergeBaseSHA {
			break
		}
		behindBy++
	}

	status := "diverged"
	if aheadBy > 0 && behindBy == 0 {
		status = "ahead"
	} else if aheadBy == 0 && behindBy > 0 {
		status = "behind"
	} else if aheadBy == 0 && behindBy == 0 {
		status = "identical"
	}

	return BranchComparison{
		AheadBy:  aheadBy,
		BehindBy: behindBy,
		Status:   status,
	}, nil
}

func NewGitHubProvider(token string, remoteOriginURL string, remoteUpstreamURL string) *GithubProvider {
	ctx := context.Background()
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tokenClient := oauth2.NewClient(ctx, tokenSource)

	return &GithubProvider{
		client:            github.NewClient(tokenClient),
		remoteOriginURL:   remoteOriginURL,
		remoteUpstreamURL: remoteUpstreamURL,
	}
}

func NewGitlabProvider(token string, hostURL string, remoteOriginURL string, remoteUpstreamURL string) (*GitlabProvider, error) {
	client, err := gitlab.NewClient(token, gitlab.WithBaseURL(hostURL))
	if err != nil {
		return nil, fmt.Errorf("failed to create gitlab client: %w", err)
	}

	return &GitlabProvider{client: client, remoteOriginURL: remoteOriginURL, remoteUpstreamURL: remoteUpstreamURL}, nil
}

type GitEntity interface {
	Format() string
}

type PullRequest struct {
	Title       string
	Author      string
	Description string
	URL         string
}

func (pr *PullRequest) Format() string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("- %s (by %s)\n  URL: %s\n  Body: %s\n\n", pr.Title, pr.Author, pr.URL, pr.Description))
	output := builder.String()
	if output == "" {
		output = "No open pull/merge requests found."
	}

	return output
}

type Release struct {
	TagName     string
	Name        string
	URL         string
	PublishedAt string
}

func (r *Release) Format() string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Release %s@%s\n  URL: %s\n\n  Published: %s\n", r.Name, r.TagName, r.URL, r.PublishedAt))
	output := builder.String()
	if output == "" {
		output = "No release info found."
	}

	return output
}

type BranchComparison struct {
	AheadBy  int
	BehindBy int
	Status   string
}

func (b *BranchComparison) Format() string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Local branch vs main branch:\n  Status: %s\n  AheadBy: %d\n  BehindBy: %d\n", b.Status, b.AheadBy, b.BehindBy))
	output := builder.String()
	if output == "" {
		output = "No branch comparison info between local and remote/upstream found."
	}

	return output
}
