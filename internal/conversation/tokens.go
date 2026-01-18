package conversation

import (
	"github.com/pkoukk/tiktoken-go"
)

// TokenCounter handles token counting for different models
type TokenCounter struct {
	// Cache encoders for reuse
	encoders map[string]*tiktoken.Tiktoken
}

// NewTokenCounter creates a new token counter
func NewTokenCounter() *TokenCounter {
	return &TokenCounter{
		encoders: make(map[string]*tiktoken.Tiktoken),
	}
}

// Count returns the number of tokens in a text for a given model
func (tc *TokenCounter) Count(text, model string) (int, error) {
	// Get encoding for model
	encoding := tc.getEncodingName(model)

	// Get or create encoder
	encoder, ok := tc.encoders[encoding]
	if !ok {
		var err error
		encoder, err = tiktoken.GetEncoding(encoding)
		if err != nil {
			// Fallback to simple estimation if tiktoken fails
			return tc.estimateTokens(text), nil
		}
		tc.encoders[encoding] = encoder
	}

	// Count tokens
	tokens := encoder.Encode(text, nil, nil)
	return len(tokens), nil
}

// CountMessages counts tokens for a slice of messages
func (tc *TokenCounter) CountMessages(messages []Message, model string) (int, error) {
	total := 0

	for _, msg := range messages {
		// Count message content
		count, err := tc.Count(msg.Content, model)
		if err != nil {
			return 0, err
		}

		// Add tokens for message formatting (role, etc.)
		// OpenAI format adds ~4 tokens per message for formatting
		total += count + 4
	}

	// Add 3 tokens for reply priming (assistant: )
	total += 3

	return total, nil
}

// getEncodingName returns the tiktoken encoding name for a model
func (tc *TokenCounter) getEncodingName(model string) string {
	// Map model names to tiktoken encodings
	// This is a simplified version - expand as needed

	// GPT-4, GPT-3.5-turbo use cl100k_base
	if contains(model, "gpt-4") || contains(model, "gpt-3.5") || contains(model, "gpt-35") {
		return "cl100k_base"
	}

	// Claude models also use cl100k_base (approximate)
	if contains(model, "claude") {
		return "cl100k_base"
	}

	// Default to cl100k_base (used by most modern models)
	return "cl100k_base"
}

// estimateTokens provides a rough token estimate (chars/4)
func (tc *TokenCounter) estimateTokens(text string) int {
	// Rough estimate: 1 token ≈ 4 characters
	return (len(text) + 3) / 4
}

// contains checks if a string contains a substring (case-insensitive helper)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
