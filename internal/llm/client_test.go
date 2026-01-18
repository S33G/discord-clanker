package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/s33g/discord-prompter/internal/config"
)

func TestClient_Chat(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("Expected /chat/completions, got %s", r.URL.Path)
		}

		// Parse request
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		// Verify request contents
		if req.Model != "test-model" {
			t.Errorf("Expected model test-model, got %s", req.Model)
		}
		if len(req.Messages) == 0 {
			t.Error("Expected messages in request")
		}

		// Send mock response
		resp := ChatResponse{
			ID:      "test-123",
			Model:   "test-model",
			Created: 1234567890,
			Choices: []Choice{
				{
					Index: 0,
					Message: Message{
						Role:    "assistant",
						Content: "This is a test response.",
					},
					FinishReason: "stop",
				},
			},
			Usage: Usage{
				PromptTokens:     10,
				CompletionTokens: 8,
				TotalTokens:      18,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create client
	provider := &config.Provider{
		Name:    "test",
		BaseURL: server.URL,
	}
	client, err := NewClient(provider)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	// Make request
	ctx := context.Background()
	req := ChatRequest{
		Model: "test-model",
		Messages: []Message{
			{Role: "user", Content: "Hello!"},
		},
	}

	resp, err := client.Chat(ctx, req)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	// Verify response
	if resp.Model != "test-model" {
		t.Errorf("Model = %v, want test-model", resp.Model)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("Choices = %d, want 1", len(resp.Choices))
	}
	if resp.Choices[0].Message.Content != "This is a test response." {
		t.Errorf("Content = %v, want 'This is a test response.'", resp.Choices[0].Message.Content)
	}
	if resp.Usage.TotalTokens != 18 {
		t.Errorf("TotalTokens = %d, want 18", resp.Usage.TotalTokens)
	}
}

func TestClient_ChatError(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		resp := ErrorResponse{}
		resp.Error.Message = "Invalid request"
		resp.Error.Type = "invalid_request_error"
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create client
	provider := &config.Provider{
		Name:    "test",
		BaseURL: server.URL,
	}
	client, _ := NewClient(provider)

	// Make request
	ctx := context.Background()
	req := ChatRequest{
		Model:    "test-model",
		Messages: []Message{{Role: "user", Content: "Hello!"}},
	}

	_, err := client.Chat(ctx, req)
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestClient_GenerateTitle(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ChatResponse{
			Choices: []Choice{
				{
					Message: Message{
						Content: "Docker Networking Basics",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create client
	provider := &config.Provider{
		Name:    "test",
		BaseURL: server.URL,
	}
	client, _ := NewClient(provider)

	// Generate title
	ctx := context.Background()
	title, err := client.GenerateTitle(ctx, "test-model", "Explain Docker networking")
	if err != nil {
		t.Fatalf("GenerateTitle() error = %v", err)
	}

	if title != "Docker Networking Basics" {
		t.Errorf("Title = %v, want 'Docker Networking Basics'", title)
	}
}

func TestClient_GenerateTitleFallback(t *testing.T) {
	// Create mock server that fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Create client
	provider := &config.Provider{
		Name:    "test",
		BaseURL: server.URL,
	}
	client, _ := NewClient(provider)

	// Generate title (should fallback gracefully)
	ctx := context.Background()
	prompt := "This is a very long prompt that should be truncated to 80 characters maximum when used as a fallback title"
	title, err := client.GenerateTitle(ctx, "test-model", prompt)

	// Should not error (falls back)
	if err != nil {
		t.Errorf("GenerateTitle() should not error on failure, got %v", err)
	}

	// Should be truncated
	if len(title) > 100 {
		t.Errorf("Title length = %d, should be <= 100", len(title))
	}
}

func TestRegistry_GetClient(t *testing.T) {
	cfg := &config.Config{
		Providers: []config.Provider{
			{
				Name:    "test1",
				BaseURL: "http://localhost:8080",
				Models: []config.Model{
					{ID: "model1", DisplayName: "Model 1"},
				},
			},
			{
				Name:    "test2",
				BaseURL: "http://localhost:8081",
				Models: []config.Model{
					{ID: "model2", DisplayName: "Model 2"},
				},
			},
		},
	}

	registry, err := NewRegistry(cfg)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	// Get existing client
	client, err := registry.GetClient("test1")
	if err != nil {
		t.Errorf("GetClient(test1) error = %v", err)
	}
	if client == nil {
		t.Error("GetClient(test1) returned nil")
	}

	// Get non-existent client
	_, err = registry.GetClient("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent provider")
	}
}
