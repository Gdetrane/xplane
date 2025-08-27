package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func pickLLM(cfg *Config) (LLMProvider, error) {
	switch cfg.Provider {
	case "claude_code":
		return &ClaudeCode{model: cfg.Model}, nil
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
	case "ollama":
		host := cfg.OllamaServerAddress
		if host == "" {
			host = "http://localhost:11434"
		}
		model := cfg.Model
		if model == "" {
			model = "gemma3n"
		}
		return &Ollama{
			serverAddress: host,
			model:         model,
		}, nil
	default:
		return nil, fmt.Errorf("xplane: unknown llm provider '%s' found in config", cfg.Provider)
	}
}

type LLMProvider interface {
	summarizeContext(finalPrompt string) (string, error)
	getName() string
}

// getKnowledgeFilePath returns the path to the shared project knowledge file
func getKnowledgeFilePath() (string, error) {
	projRoot, err := findGitRoot()
	if err != nil {
		return "", err
	}
	xplaneDir := filepath.Join(projRoot, ".xplane")
	if err := os.MkdirAll(xplaneDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(xplaneDir, "KNOWLEDGE.md"), nil
}

type ClaudeCode struct {
	model string
}

func (c *ClaudeCode) getName() string {
	return "Claude Code"
}

func (c *ClaudeCode) summarizeContext(finalPrompt string) (string, error) {
	args := []string{"--print", "--model", c.model}
	cmd := exec.Command("claude", args...)

	cmd.Stdin = strings.NewReader(finalPrompt)

	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("claude code failed with args %v: %v, stderr: %v", args, err, stderr.String())
	}
	return out.String(), nil
}

type GeminiCli struct {
	model string
}

func (g *GeminiCli) getName() string {
	return "Gemini CLI"
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

func (g *Gemini) getName() string {
	return "Gemini"
}

func (g *Gemini) summarizeContext(finalPrompt string) (string, error) {
	return "Summary from Gemini (not the same as Gemini CLI!) not implemented yet", nil
}

type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type OllamaResponse struct {
	Response string `json:"response"`
}

type OllamaModelInfo struct {
	Name string `json:"name"`
}

type OllamaTagsResponse struct {
	Models []OllamaModelInfo `json:"models"`
}

type Ollama struct {
	serverAddress string
	model         string
}

func (o *Ollama) getName() string {
	return "Ollama"
}

func (o *Ollama) checkModelAvailability() (bool, error) {
	apiEndpoint := o.serverAddress + "/api/tags"
	resp, err := http.Get(apiEndpoint)
	if err != nil {
		return false, fmt.Errorf("could not connect to ollama server at '%s': %w. Is the server running?", o.serverAddress, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("ollama server returned non-200 status: %s", resp.Status)
	}

	var tagsResponse OllamaTagsResponse
	decodingErr := json.NewDecoder(resp.Body).Decode(&tagsResponse)
	if decodingErr != nil {
		return false, fmt.Errorf("failed to decode ollama tags response: %w", decodingErr)
	}

	for _, model := range tagsResponse.Models {
		if strings.HasPrefix(model.Name, o.model) {
			return true, nil
		}
	}

	return false, nil
}

func (o *Ollama) summarizeContext(finalPrompt string) (string, error) {
	// before even attempting to prompt the model, let's check it's been pulled
	modelIsPulled, err := o.checkModelAvailability()
	if err != nil {
		return "", err
	}

	if !modelIsPulled {
		return "", fmt.Errorf("ollama model '%s' not found. Please pull it by running 'ollama pull %s' on the host server.", o.model, o.model)
	}
	requestPayload := OllamaRequest{
		Model:  o.model,
		Prompt: finalPrompt,
		Stream: false,
	}

	payloadBytes, err := json.Marshal(requestPayload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal ollama request: %w", err)
	}

	apiEndpoint := o.serverAddress + "/api/generate"
	req, err := http.NewRequest("POST", apiEndpoint, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request to ollama server '%s': %w", o.serverAddress, err)
	}
	defer resp.Body.Close()

	var ollamaResponse OllamaResponse
	decodingErr := json.NewDecoder(resp.Body).Decode(&ollamaResponse)
	if decodingErr != nil {
		return "", fmt.Errorf("failed to decode ollama response: %w", decodingErr)
	}

	return ollamaResponse.Response, nil
}
