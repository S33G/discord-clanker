package conversation

import (
	"testing"
)

func TestContextBuilder_Build(t *testing.T) {
	cb := NewContextBuilder(100, 20) // 100 max, 20 reserved for response

	messages := []Message{
		{Role: "user", Content: "First message", Tokens: 10},
		{Role: "assistant", Content: "First response", Tokens: 10},
		{Role: "user", Content: "Second message", Tokens: 10},
		{Role: "assistant", Content: "Second response", Tokens: 10},
		{Role: "user", Content: "Third message", Tokens: 10},
	}

	result, total, err := cb.Build(messages, "You are helpful.", "gpt-4")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	// Should have system prompt
	if result[0].Role != "system" {
		t.Error("First message should be system prompt")
	}

	// Total should be reasonable
	if total <= 0 || total > 100 {
		t.Errorf("Total tokens = %d, expected between 1 and 100", total)
	}

	// Messages should be in chronological order (after system)
	if len(result) > 2 {
		for i := 2; i < len(result); i++ {
			// Each user message should come before its response
			if result[i].Role == "assistant" && result[i-1].Role != "user" {
				t.Error("Messages out of order")
			}
		}
	}
}

func TestContextBuilder_TruncatesOldMessages(t *testing.T) {
	cb := NewContextBuilder(50, 10) // Very limited budget

	messages := []Message{
		{Role: "user", Content: "Old message", Tokens: 15},
		{Role: "assistant", Content: "Old response", Tokens: 15},
		{Role: "user", Content: "New message", Tokens: 15},
	}

	result, total, err := cb.Build(messages, "System.", "gpt-4")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	// Should fit within budget
	if total > 50-10 {
		t.Errorf("Total tokens %d exceeds budget %d", total, 50-10)
	}

	// Should prioritize recent messages
	hasNew := false
	for _, msg := range result {
		if msg.Content == "New message" {
			hasNew = true
		}
	}
	if !hasNew {
		t.Error("Should keep most recent message")
	}
}

func TestContextBuilder_SystemPromptAlwaysIncluded(t *testing.T) {
	cb := NewContextBuilder(20, 5) // Very small budget

	messages := []Message{
		{Role: "user", Content: "This is a test", Tokens: 50}, // Too large
	}

	result, _, err := cb.Build(messages, "Be helpful.", "gpt-4")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	// Should at minimum include system prompt
	if len(result) < 1 {
		t.Fatal("Should include at least system prompt")
	}
	if result[0].Role != "system" {
		t.Error("First message should always be system prompt")
	}
}

func TestTokenCounter_Count(t *testing.T) {
	tc := NewTokenCounter()

	tests := []struct {
		name  string
		text  string
		model string
	}{
		{
			name:  "simple text",
			text:  "Hello, world!",
			model: "gpt-4",
		},
		{
			name:  "longer text",
			text:  "This is a longer piece of text that should have more tokens.",
			model: "gpt-3.5-turbo",
		},
		{
			name:  "empty text",
			text:  "",
			model: "gpt-4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := tc.Count(tt.text, tt.model)
			if err != nil {
				t.Fatalf("Count() error = %v", err)
			}

			if tt.text == "" && count != 0 {
				t.Errorf("Empty text should have 0 tokens, got %d", count)
			}

			if tt.text != "" && count <= 0 {
				t.Errorf("Non-empty text should have positive tokens, got %d", count)
			}

			// Sanity check: tokens should be roughly text_length/4 or less
			maxExpected := len(tt.text) // Each char could be a token in worst case
			if count > maxExpected {
				t.Errorf("Token count %d exceeds max expected %d", count, maxExpected)
			}
		})
	}
}

func TestTokenCounter_CountMessages(t *testing.T) {
	tc := NewTokenCounter()

	messages := []Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hello!"},
		{Role: "assistant", Content: "Hi there!"},
	}

	count, err := tc.CountMessages(messages, "gpt-4")
	if err != nil {
		t.Fatalf("CountMessages() error = %v", err)
	}

	if count <= 0 {
		t.Error("Message count should be positive")
	}

	// Should include overhead (4 tokens per message + 3 for priming)
	minExpected := len(messages)*4 + 3
	if count < minExpected {
		t.Errorf("Count %d less than minimum expected %d", count, minExpected)
	}
}
