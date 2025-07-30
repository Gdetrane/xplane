package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func pickLLM(cfg *Config) (LLMProvider, error) {
	switch cfg.Provider {
	case "gemini_cli":
		return &GeminiCli{model: cfg.Model}, nil
	case "gemini":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("xplane: Error configuring provider 'gemini', you need to provide an api key via XPLANE_API_KEY")
		}
		return &Gemini{
			model:  cfg.Model,
			apiKey: cfg.APIKey,
		}, nil
	default:
		return nil, fmt.Errorf("xplane: unknown llm provider '%s' found in config", cfg.Provider)
	}
}

type LLMProvider interface {
	summarizeContext(finalPrompt string) (string, error)
}

type GeminiCli struct {
	model string
}

func (g *GeminiCli) summarizeContext(finalPrompt string) (string, error) {
	args := []string{"-y", "-m", g.model} // see gemini --help
	cmd := exec.Command("gemini", args...)

	// passing the full prompt to stdin
	cmd.Stdin = strings.NewReader(finalPrompt)

	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()

	if err != nil {
		return "", fmt.Errorf("gemini cli failed with args %v: %v, stderr: %v", args, err, stderr.String())
	}
	return out.String(), nil
}

type Gemini struct {
	model  string
	apiKey string
}

func (g *Gemini) summarizeContext(finalPrompt string) (string, error) {
	return "Summary from Gemini (not the same as Gemini CLI!) not implemented yet", nil
}

type Ollama struct {
	serverAddress string
	model         string
}

func (o *Ollama) summarizeContext(finalPrompt string) (string, error) {
	return "Summary from Ollama (Not Implemented)", nil
}
