package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var llm = os.Getenv("LLM")

func createPlaceHolderContext(cfg *Config) string {
	var placeholderBuilder strings.Builder
	for _, command := range cfg.Commands {
		trimmedCmd := strings.TrimSpace(command)
		placeholderBuilder.WriteString(fmt.Sprintf("---CONTEXT FROM: %s ---\n%s\n\n",
			trimmedCmd, "First run, no context available yet."))
	}
	return placeholderBuilder.String()
}

// wraps around various special commands, as well as custom commands, to gather context for an LLM
func gatherContext(cfg *Config, gitRoot string) (string, error) {
	var contextBuilder strings.Builder
	var gitProvider GitProvider
	var gitProviderErr error
	var gitProviderInitialized bool

	initGitProvider := func() (GitProvider, error) {
		if !gitProviderInitialized {
			gitProvider, gitProviderErr = getGitProvider(gitRoot, cfg)
			gitProviderInitialized = true
		}
		return gitProvider, gitProviderErr
	}

	for _, command := range cfg.Commands {
		var output string
		var err error
		trimmedCmd := strings.TrimSpace(command)

		switch trimmedCmd {
		case "git_status":
			output, err = getGitStatus(gitRoot)
		case "git_log":
			output, err = getGitLog(gitRoot, 15)
		case "tokei":
			output, err = getTokeiStats(gitRoot)
		case "ripsecrets":
			output, err = getRipSecrets(gitRoot)
		case "readme":
			readmeBytes, readmeErr := os.ReadFile(filepath.Join(gitRoot, "README.md"))
			if os.IsNotExist(readmeErr) {
				output = "No README.md file provided in this project."
			} else {
				output = string(readmeBytes)
			}

		case "release":
		  provider, err := initGitProvider()
			if err != nil {
				return "", fmt.Errorf("failed to initialize git provider: %v", err)
			}
			primaryURL, urlErr := findPrimaryRemoteRepoURL(gitRoot)
			if urlErr != nil {
				return "", urlErr
			}

			owner, repo, parseErr := parseGitURL(primaryURL)
			if parseErr != nil {
				return "", parseErr
			}
		  release, releaseErr := provider.GetLatestRelease(owner, repo)
		  if releaseErr != nil {
		  	return "", releaseErr
		  }
		  output = formatReleaseInfo(release)
		  
		case "git_branch_status":
			// checking that the local branch has remote tracking first
			if !hasRemoteTrackingBranch(gitRoot) {
				output = "Local branch has not been pushed to the remote."
				break
			}
		  provider, err := initGitProvider()
			if err != nil {
				return "", fmt.Errorf("failed to initialize git provider: %v", err)
			}
			primaryURL, urlErr := findPrimaryRemoteRepoURL(gitRoot)
			if urlErr != nil {
				return "", urlErr
			}

			owner, repo, parseErr := parseGitURL(primaryURL)
			if parseErr != nil {
				return "", parseErr
			}
			localBranch, localBranchErr := runCommand(gitRoot, "git", "branch", "--show-current")
			if localBranchErr != nil {
				return "", localBranchErr
			}
			localBranch = strings.TrimSpace(localBranch)
		  branchComparison, branchComparisonErr := provider.CompareBranchWithDefault(owner, repo, localBranch)
		  if branchComparisonErr != nil {
		  	return "", branchComparisonErr
		  }
		  output = formatBranchComparison(branchComparison)

		case "github_prs", "gitlab_mrs":
			provider, err := initGitProvider()
			if err != nil {
				return "", fmt.Errorf("failed to initialize git provider: %v", err)
			}

			primaryURL, urlErr := findPrimaryRemoteRepoURL(gitRoot)
			if urlErr != nil {
				return "", urlErr
			}

			owner, repo, parseErr := parseGitURL(primaryURL)
			if parseErr != nil {
				return "", parseErr
			}

			prs, prErr := provider.GetOpenPullRequests(owner, repo)
			if prErr != nil {
				return "", prErr
			}

			output = formatPRS(prs)
		default:
			fmt.Printf("xplane: Running generic command '%s' ...\n", trimmedCmd)
			output, err = runCommand(gitRoot, trimmedCmd, gitRoot)
		}

		if err != nil {
			return "", fmt.Errorf("error running command '%s': %w", trimmedCmd, err)
		}
		contextBuilder.WriteString(fmt.Sprintf("---CONTEXT FROM: %s ---\n%s\n\n", trimmedCmd, output))
	}

	return contextBuilder.String(), nil
}

func contextCompare(llm LLMProvider, cfg *Config, gitRoot string) {
	dynamicContextPath := filepath.Join(gitRoot, contextDir, dynamicContextFile)
	staticContextPath := filepath.Join(gitRoot, contextDir, staticContextFile)

	fetchedDynamicContext, err := gatherContext(cfg, gitRoot)
	if err != nil {
		log.Fatalf("xplane: Error gathering context: %v", err)
	}

	previousDynamicContext, err := os.ReadFile(dynamicContextPath)
	if os.IsNotExist(err) {
		fmt.Println("xplane: Initializing project. No summary will be generated on this first run.")
		placeholderContext := createPlaceHolderContext(cfg)
		os.MkdirAll(filepath.Dir(dynamicContextPath), 0o755)
		os.WriteFile(dynamicContextPath, []byte(placeholderContext), 0o644)
		return
	}

	if fetchedDynamicContext == string(previousDynamicContext) {
		fmt.Println("✅ xplane: No new updates.")
		return
	}

	// always writing to the file if there are changes in dynamic context
	defer func() {
		os.MkdirAll(filepath.Dir(dynamicContextPath), 0o755)
		os.WriteFile(dynamicContextPath, []byte(fetchedDynamicContext), 0o644)
		fmt.Println("xplane: Context updated.")
	}()

	// reading the static prompt template and ensuring it's built
	staticPromptBytes, err := os.ReadFile(staticContextPath)
	if os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(staticContextPath), 0o755)
		os.WriteFile(staticContextPath, []byte(defaultStaticContext), 0o644)
	}
	if err != nil {
		log.Fatalf("xplane: Could not read static context template file: %v", err)
	}
	staticPrompt := string(staticPromptBytes)

	finalPrompt := strings.Replace(staticPrompt, "{{CURRENT_CONTEXT}}", fetchedDynamicContext, -1)
	finalPrompt = strings.Replace(finalPrompt, "{{PREVIOUS_CONTEXT}}", string(previousDynamicContext), -1)

	// getting summary from LLM
	summary, err := llm.summarizeContext(finalPrompt)
	if err != nil {
		fmt.Printf("⚠️ xplane: Could not generate summary: %v\n", err)
	} else {
		fmt.Println(summary)
	}
}
