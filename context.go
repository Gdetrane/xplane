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
	fmt.Println(MsgFetchingContext)
	var contextBuilder strings.Builder

	gatherer := NewContextGatherer(gitRoot, cfg)
	initErr := gatherer.initProvider()

	commandHandlersMap := map[string]func() (string, error){
		"git_status":        func() (string, error) { return getGitStatus(gitRoot) },
		"git_log":           func() (string, error) { return getGitLog(gitRoot, 15) },
		"tokei":             func() (string, error) { return getTokeiStats(gitRoot) },
		"ripsecrets":        func() (string, error) { return getRipSecrets(gitRoot) },
		"readme":            func() (string, error) { return getReadme(gitRoot) },
		"git_exclude":       func() (string, error) { return getGitExclude(gitRoot) },
		"gitignore":         func() (string, error) { return getGitignore(gitRoot) },
		"git_diff":          func() (string, error) { return getGitDiff(gitRoot) },
		"github_prs":        gatherer.getOpenPRS,
		"gitlab_mrs":        gatherer.getOpenPRS,
		"release":           gatherer.getLatestRelease,
		"git_branch_status": gatherer.getGitBranchStatus,
	}

	for _, command := range cfg.Commands {
		var output string
		var err error
		trimmedCmd := strings.TrimSpace(command)

		isGitProviderBasedCommand := trimmedCmd == "github_prs" || trimmedCmd == "gitlab_mrs" || trimmedCmd == "release" || trimmedCmd == "git_branch_status"

		if isGitProviderBasedCommand {
			if initErr != nil {
				fmt.Printf("    - ⚠️  Skipping command '%s': could not initialize git provider (%v)\n", trimmedCmd, initErr)
				continue
			}
			providerName := gatherer.gitProvider.GetProviderName()
			if providerName == "github" && trimmedCmd == "gitlab_mrs" {
				continue
			}
			if providerName == "gitlab" && trimmedCmd == "github_prs" {
				continue
			}
			fmt.Println(buildRemoteInfoMsg(providerName, trimmedCmd))
		}

		if handler, ok := commandHandlersMap[trimmedCmd]; ok {
			output, err = handler()
		} else {
			fmt.Printf(MsgGenericCommand, trimmedCmd)
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
	// reading the static prompt template
	staticPromptBytes, err := os.ReadFile(staticContextPath)
	if os.IsNotExist(err) {
		fmt.Println("xplane: static_context.txt not found, creating default.")
		if err := os.MkdirAll(filepath.Dir(staticContextPath), 0o755); err != nil {
			log.Fatalf("xplane: could not create .xplane directory: %v", err)
		}
		if err := os.WriteFile(staticContextPath, []byte(defaultStaticContext), 0o644); err != nil {
			log.Fatalf("xplane: could not write default static context: %v", err)
		}
		staticPromptBytes, err = os.ReadFile(staticContextPath)
	}

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
	fmt.Printf(MsgAnalyzingContext, llm.getName(), cfg.Model)

	// always writing to the file if there are changes in dynamic context
	defer func() {
		os.MkdirAll(filepath.Dir(dynamicContextPath), 0o755)
		os.WriteFile(dynamicContextPath, []byte(fetchedDynamicContext), 0o644)
		fmt.Println("xplane: Context updated.")
	}()

	// reading the static prompt template and ensuring it's built
	staticPrompt := string(staticPromptBytes)

	finalPrompt := strings.ReplaceAll(staticPrompt, "{{CURRENT_CONTEXT}}", fetchedDynamicContext)
	finalPrompt = strings.ReplaceAll(finalPrompt, "{{PREVIOUS_CONTEXT}}", string(previousDynamicContext))

	// getting summary from LLM
	summary, err := llm.summarizeContext(finalPrompt)
	if err != nil {
		fmt.Printf("⚠️ xplane: Could not generate summary: %v\n", err)
	} else {
		renderedSummary, renderErr := renderMarkdown(summary)
		if renderErr != nil {
			// fallback to printing
			fmt.Println("Error rendering markdown, printing raw output:")
			fmt.Println(summary)
		} else {
			fmt.Println(renderedSummary)
		}
	}
}
