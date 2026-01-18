package llm

import (
	"context"
	"fmt"
	"sync"

	"github.com/s33g/discord-prompter/internal/config"
)

// Registry manages LLM providers and their clients
type Registry struct {
	clients map[string]*Client // key: provider name
	mu      sync.RWMutex
	config  *config.Config
}

// NewRegistry creates a new provider registry
func NewRegistry(cfg *config.Config) (*Registry, error) {
	r := &Registry{
		clients: make(map[string]*Client),
		config:  cfg,
	}

	// Initialize clients for all providers
	for i := range cfg.Providers {
		client, err := NewClient(&cfg.Providers[i])
		if err != nil {
			return nil, fmt.Errorf("failed to create client for provider %s: %w", cfg.Providers[i].Name, err)
		}
		r.clients[cfg.Providers[i].Name] = client
	}

	return r, nil
}

// GetClient returns the client for a provider
func (r *Registry) GetClient(providerName string) (*Client, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	client, ok := r.clients[providerName]
	if !ok {
		return nil, fmt.Errorf("provider %s not found", providerName)
	}

	return client, nil
}

// Chat sends a chat request to the appropriate provider
func (r *Registry) Chat(ctx context.Context, modelRef string, messages []Message, maxTokens int, temperature float64) (*ChatResponse, error) {
	// Resolve model reference (e.g., "openai/gpt-4o")
	provider, model, err := r.config.ResolveModel(modelRef)
	if err != nil {
		return nil, err
	}

	// Get client
	client, err := r.GetClient(provider.Name)
	if err != nil {
		return nil, err
	}

	// Prepare request
	req := ChatRequest{
		Model:       model.ID,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: temperature,
	}

	// Send request
	return client.Chat(ctx, req)
}

// GenerateTitle generates a title for a conversation
func (r *Registry) GenerateTitle(ctx context.Context, modelRef, userPrompt string) (string, error) {
	// Resolve model reference
	provider, model, err := r.config.ResolveModel(modelRef)
	if err != nil {
		return "", err
	}

	// Get client
	client, err := r.GetClient(provider.Name)
	if err != nil {
		return "", err
	}

	return client.GenerateTitle(ctx, model.ID, userPrompt)
}

// Reload reinitializes clients after config reload
func (r *Registry) Reload(cfg *config.Config) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Create new clients
	newClients := make(map[string]*Client)
	for i := range cfg.Providers {
		client, err := NewClient(&cfg.Providers[i])
		if err != nil {
			return fmt.Errorf("failed to create client for provider %s: %w", cfg.Providers[i].Name, err)
		}
		newClients[cfg.Providers[i].Name] = client
	}

	// Replace clients
	r.clients = newClients
	r.config = cfg

	return nil
}
