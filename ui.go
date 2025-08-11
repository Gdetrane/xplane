package main

import (
	"fmt"

	"github.com/charmbracelet/glamour"
)

const xplaneHeader = `
██╗  ██╗██████╗ ██╗      █████╗ ███╗   ██╗███████╗
╚██╗██╔╝██╔══██╗██║     ██╔══██╗████╗  ██║██╔════╝
 ╚███╔╝ ██████╔╝██║     ███████║██╔██╗ ██║█████╗  
 ██╔██╗ ██╔═══╝ ██║     ██╔══██║██║╚██╗██║██╔══╝  
██╔╝ ██╗██║     ███████╗██║  ██║██║ ╚████║███████╗
╚═╝  ╚═╝╚═╝     ╚══════╝╚═╝  ╚═╝╚═╝  ╚═══╝╚══════╝
                                                  
`

// formats a raw markdown string and renders it in a terminal environment
func renderMarkdown(rawMarkdown string) (string, error) {
	fullContent := fmt.Sprintf("```\n%s\n```\n\n%s", xplaneHeader, rawMarkdown)
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dracula"),
		glamour.WithWordWrap(0), // setting to 0 lets the terminal emulator handle it
	)
	if err != nil {
		return "", err
	}

	return renderer.Render(fullContent)
}
