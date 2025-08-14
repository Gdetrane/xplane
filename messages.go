package main

import "fmt"

const (
	MsgFetchingContext          = "✈️  xplane: Gathering project context..."
	MsgGenericCommand           = "    - \ue795     Running generic command '%s' ...\n"
	MsgGetCodeStats             = "    - \ueb03     Analyzing code stats..."
	MsgGetLeakedSecrets         = "    - \uf43d     Detecting potentially leaked secrets..."
	MsgCheckingGitStatus        = "    - \ue65d     Checking local git status..."
	MsgFetchingGitLog           = "    - \ue65d     Fetching recent git log..."
	MsgFetchingGitDiff          = "    - \ue65d     Fetching uncommitted diff..."
	MsgFetchingGithubRemoteInfo = "    - \uF09B     Fetching info from GitHub: %s"
	MsgFetchingGitlabRemoteInfo = "    - \ue65c     Fetching info from GitLab: %s"
	MsgAnalyzingContext         = "\uee0d  xplane: Context has changed, analyzing with %s provider using '%s'...\n\n\n"
)

func buildRemoteInfoMsg(providerName string, commandName string) string {
	switch providerName {
	case "github":
		if commandName == "release" {
			return fmt.Sprintf(MsgFetchingGithubRemoteInfo, "Getting latest release...")
		}
		if commandName == "github_prs" {
			return fmt.Sprintf(MsgFetchingGithubRemoteInfo, "Getting open PRs...")
		}
		if commandName == "git_branch_status" {
			return fmt.Sprintf(MsgFetchingGithubRemoteInfo, "Comparing current branch to upstream...")
		}
	case "gitlab":
		if commandName == "release" {
			return fmt.Sprintf(MsgFetchingGitlabRemoteInfo, "Getting latest release...")
		}
		if commandName == "gitlab_mrs" {
			return fmt.Sprintf(MsgFetchingGitlabRemoteInfo, "Getting open MRs...")
		}
		if commandName == "git_branch_status" {
			return fmt.Sprintf(MsgFetchingGitlabRemoteInfo, "Comparing current branch to upstream...")
		}
	default:
		return fmt.Sprintf("Unexpected command: %s", commandName)
	}
	return fmt.Sprintf("Unexpected git provider: %s", providerName)
}
