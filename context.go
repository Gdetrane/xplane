package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
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

	// inject project knowledge instructions if enabled
	if cfg.UseProjectKnowledge {
		knowledgeContent, knowledgeErr := readKnowledgeFile()
		if knowledgeErr != nil {
			log.Printf("Warning: Could not read knowledge file: %v", knowledgeErr)
			knowledgeContent = "No existing project knowledge found."
		}

		knowledgeSection := fmt.Sprintf(`

--- PROJECT KNOWLEDGE ---
%s

CRITICAL KNOWLEDGE MANAGEMENT INSTRUCTIONS:
This project maintains a living knowledge base at .xplane/KNOWLEDGE.md that must grow over time.

Current knowledge above represents the institutional memory of this project. Your task is to:

1. READ the existing knowledge carefully - it contains important context about the project's evolution
2. ANALYZE the current changes in relation to this existing knowledge
3. If this session reveals any of the following, you MUST include a 'KNOWLEDGE UPDATE' section:
   - New architectural decisions or technology stack changes
   - Important bug fixes or patterns discovered
   - Significant feature additions or modifications
   - Development workflow changes
   - Dependencies or configuration changes
   - Any insights that would help future development sessions

KNOWLEDGE UPDATE format:
- Include a 'KNOWLEDGE UPDATE' section in your response containing ONLY NEW insights
- Focus on what's NEW or CHANGED since the last session
- DO NOT repeat existing knowledge - the system will preserve it automatically
- Organize new insights by: Architecture, Recent Changes, Important Patterns, Development Notes
- Be comprehensive about NEW information that would help future development sessions

Your KNOWLEDGE UPDATE should contain only fresh insights - existing knowledge will be preserved automatically in a timeline format.`, knowledgeContent)

		staticPrompt = staticPrompt + knowledgeSection
	}

	finalPrompt := strings.ReplaceAll(staticPrompt, "{{CURRENT_CONTEXT}}", fetchedDynamicContext)
	finalPrompt = strings.ReplaceAll(finalPrompt, "{{PREVIOUS_CONTEXT}}", string(previousDynamicContext))

	// getting summary from LLM
	summary, err := llm.summarizeContext(finalPrompt)
	if err != nil {
		fmt.Printf("⚠️ xplane: Could not generate summary: %v\n", err)
	} else {
		// handle knowledge updates if enabled
		if cfg.UseProjectKnowledge {
			if updatedKnowledge := extractKnowledgeUpdate(summary); updatedKnowledge != "" {
				if err := writeKnowledgeFile(updatedKnowledge); err != nil {
					log.Printf("Warning: Could not update knowledge file: %v", err)
				} else {
					fmt.Println(MsgKnowledgeUpdated)
				}
			}
		}

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

// readKnowledgeFile reads the project knowledge file content
func readKnowledgeFile() (string, error) {
	knowledgePath, err := getKnowledgeFilePath()
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(knowledgePath)
	if os.IsNotExist(err) {
		// Initialize empty knowledge file on first run
		initialContent := "*This file will be automatically updated with project insights and important context.*"
		if err := writeKnowledgeFile(initialContent); err != nil {
			return "", fmt.Errorf("failed to initialize knowledge file: %v", err)
		}
		fmt.Println(MsgKnowledgeInitialized)
		// Return the timestamped content that was actually written
		return fmt.Sprintf("# Project Knowledge\n\n*Last updated: %s*\n\n%s", time.Now().Format("2006-01-02 15:04:05"), initialContent), nil
	}
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// writeKnowledgeFile prepends new content to the project knowledge file with timestamp
func writeKnowledgeFile(newContent string) error {
	knowledgePath, err := getKnowledgeFilePath()
	if err != nil {
		return err
	}

	// Read existing content if file exists
	var existingContent string
	if existingData, err := os.ReadFile(knowledgePath); err == nil {
		existingContent = string(existingData)
		// Remove the header and last updated line from existing content for clean prepending
		lines := strings.Split(existingContent, "\n")
		if len(lines) >= 3 && strings.HasPrefix(lines[0], "# Project Knowledge") {
			// Skip header (line 0), empty line (line 1), and last updated line (line 2)
			existingContent = strings.Join(lines[3:], "\n")
			existingContent = strings.TrimSpace(existingContent)
		}
	}

	// Create the new timestamped entry
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	var finalContent string

	if existingContent == "" || existingContent == "*This file will be automatically updated with project insights and important context.*" {
		// First real content - no existing knowledge to preserve
		finalContent = fmt.Sprintf("# Project Knowledge\n\n*Last updated: %s*\n\n%s", timestamp, newContent)
	} else {
		// Prepend new content to existing content
		finalContent = fmt.Sprintf("# Project Knowledge\n\n*Last updated: %s*\n\n## Latest Update (%s)\n\n%s\n\n---\n\n## Previous Knowledge\n\n%s", 
			timestamp, timestamp, newContent, existingContent)
	}

	return os.WriteFile(knowledgePath, []byte(finalContent), 0644)
}

// extractKnowledgeUpdate extracts knowledge update from LLM response
func extractKnowledgeUpdate(response string) string {
	lines := strings.Split(response, "\n")
	var inKnowledgeSection bool
	var knowledgeLines []string
	var foundHeader bool

	for i, line := range lines {
		// Look for the KNOWLEDGE UPDATE header (various formats)
		if !foundHeader && (strings.Contains(strings.ToUpper(line), "KNOWLEDGE UPDATE") ||
			strings.Contains(strings.ToUpper(line), "## KNOWLEDGE UPDATE") ||
			strings.Contains(strings.ToUpper(line), "### KNOWLEDGE UPDATE")) {
			inKnowledgeSection = true
			foundHeader = true
			continue
		}

		if inKnowledgeSection {
			// Skip empty lines immediately after the header until we find content
			if len(knowledgeLines) == 0 && strings.TrimSpace(line) == "" {
				continue
			}

			// More conservative stopping conditions - only stop on clear section boundaries
			trimmedLine := strings.TrimSpace(line)
			if (strings.Contains(strings.ToUpper(line), "UNCERTAINTY MAP") && len(knowledgeLines) > 0) ||
				(strings.HasPrefix(trimmedLine, "## ") && !strings.Contains(strings.ToUpper(trimmedLine), "KNOWLEDGE") && len(knowledgeLines) > 3) ||
				(strings.HasPrefix(trimmedLine, "# ") && !strings.Contains(strings.ToUpper(trimmedLine), "KNOWLEDGE") && len(knowledgeLines) > 3) {
				break
			}

			// Add the line to knowledge content
			knowledgeLines = append(knowledgeLines, line)

			// If this is the last line of the response, we're done
			if i == len(lines)-1 {
				break
			}
		}
	}

	if len(knowledgeLines) == 0 {
		return ""
	}

	// Clean up trailing empty lines
	for len(knowledgeLines) > 0 && strings.TrimSpace(knowledgeLines[len(knowledgeLines)-1]) == "" {
		knowledgeLines = knowledgeLines[:len(knowledgeLines)-1]
	}

	result := strings.TrimSpace(strings.Join(knowledgeLines, "\n"))

	// Safeguard: if the extracted knowledge is suspiciously short (less than 50 chars),
	// it's probably incomplete - don't update
	if len(result) < 50 {
		log.Printf("Warning: Knowledge update too short (%d chars), skipping to prevent data loss", len(result))
		return ""
	}

	return result
}
