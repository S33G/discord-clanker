package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/s33g/discord-prompter/internal/config"
)

// Client handles communication with LLM providers
type Client struct {
	httpClient *http.Client
	provider   *config.Provider
	apiKey     string
}

// NewClient creates a new LLM client for a provider
func NewClient(provider *config.Provider) (*Client, error) {
	// Get API key from environment if specified
	apiKey := ""
	if provider.APIKeyEnv != "" {
		apiKey = os.Getenv(provider.APIKeyEnv)
		// API key is optional (e.g., for local Ollama)
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: 120 * time.Second, // 2 minute timeout for LLM requests
		},
		provider: provider,
		apiKey:   apiKey,
	}, nil
}

// Chat sends a chat completion request
func (c *Client) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Build request URL
	url := c.provider.BaseURL + "/chat/completions"

	// Marshal request
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &chatResp, nil
}

// GenerateTitle generates a short title for a conversation
func (c *Client) GenerateTitle(ctx context.Context, model, userPrompt string) (string, error) {
	req := ChatRequest{
		Model: model,
		Messages: []Message{
			{
				Role:    "system",
				Content: "Generate a short, descriptive title (max 100 characters) for a conversation that starts with the following message. Reply with ONLY the title, no quotes or formatting.",
			},
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
		MaxTokens:   50,
		Temperature: 0.7,
	}

	resp, err := c.Chat(ctx, req)
	if err != nil {
		// Fallback to first 80 chars of prompt
		title := userPrompt
		if len(title) > 80 {
			title = title[:77] + "..."
		}
		return title, nil // Don't fail the whole request if title generation fails
	}

	if len(resp.Choices) == 0 {
		return userPrompt[:min(80, len(userPrompt))], nil
	}

	title := resp.Choices[0].Message.Content

	// Trim title if too long
	if len(title) > 100 {
		title = title[:97] + "..."
	}

	return title, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
