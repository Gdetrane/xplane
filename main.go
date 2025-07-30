package main

import (
	"log"
)

const (
	contextDir           = ".xplane"
	dynamicContextFile   = "dynamic_context.txt"
	staticContextFile    = "static_context.txt"
	defaultStaticContext = `
		You are a helpful project assistant. Your goal is to provide a clear and concise summary of the project's changes.

		Summarize the key differences between the PREVIOUS and CURRENT states provided below.

		--- PREVIOUS STATE ---
		{{PREVIOUS_CONTEXT}}

		--- CURRENT STATE ---
		{{CURRENT_CONTEXT}}

		---
		Add a section at the end of your responses labeled 'UNCERTAINTY MAP', where you describe what you're least confident about and what questions would change your opinion.
	`
)

func main() {
	gitRoot, err := findGitRoot()
	if err != nil {
		log.Fatalf("Error: not inside a git repository. %v", err)
	}

	// loading configuration
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	llmProvider, err := pickLLM(cfg)
	if err != nil {
		log.Fatalf("Error loading an llm provider: %v", err)
	}

	contextCompare(llmProvider, cfg, gitRoot)
}
