package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Load reads and parses the configuration file
func Load(path string) (*Config, error) {
	// Start with defaults
	cfg := DefaultConfig()

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Get Discord token from environment
	cfg.Discord.Token = os.Getenv("DISCORD_TOKEN")
	if cfg.Discord.Token == "" {
		return nil, fmt.Errorf("DISCORD_TOKEN environment variable is required")
	}

	// Validate
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

// Validate checks that the configuration is valid
func (c *Config) Validate() error {
	// Validate Redis
	if c.Redis.Address == "" {
		return fmt.Errorf("redis.address is required")
	}

	// Validate providers
	if len(c.Providers) == 0 {
		return fmt.Errorf("at least one provider is required")
	}

	providerModels := make(map[string]bool)
	for i, provider := range c.Providers {
		if provider.Name == "" {
			return fmt.Errorf("provider[%d].name is required", i)
		}
		if provider.BaseURL == "" {
			return fmt.Errorf("provider[%d].base_url is required", i)
		}
		if len(provider.Models) == 0 {
			return fmt.Errorf("provider[%d] must have at least one model", i)
		}

		for j, model := range provider.Models {
			if model.ID == "" {
				return fmt.Errorf("provider[%d].models[%d].id is required", i, j)
			}
			if model.DisplayName == "" {
				return fmt.Errorf("provider[%d].models[%d].display_name is required", i, j)
			}

			// Track provider/model combinations
			fullID := fmt.Sprintf("%s/%s", provider.Name, model.ID)
			providerModels[fullID] = true
		}
	}

	// Validate guilds
	if len(c.Guilds) == 0 {
		return fmt.Errorf("at least one guild is required")
	}

	for i, guild := range c.Guilds {
		if guild.ID == "" {
			return fmt.Errorf("guilds[%d].id is required", i)
		}
		if len(guild.EnabledModels) == 0 {
			return fmt.Errorf("guilds[%d].enabled_models is required", i)
		}
		if guild.DefaultModel == "" {
			return fmt.Errorf("guilds[%d].default_model is required", i)
		}

		// Validate enabled models reference valid providers
		for _, modelRef := range guild.EnabledModels {
			if !providerModels[modelRef] {
				return fmt.Errorf("guilds[%d].enabled_models references unknown model: %s", i, modelRef)
			}
		}

		// Validate default model is in enabled models
		found := false
		for _, modelRef := range guild.EnabledModels {
			if modelRef == guild.DefaultModel {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("guilds[%d].default_model must be in enabled_models", i)
		}

		// Validate system prompts
		if len(guild.SystemPrompts) == 0 {
			return fmt.Errorf("guilds[%d] must have at least one system prompt", i)
		}

		// Validate RBAC
		if len(guild.RBAC.Roles) == 0 {
			return fmt.Errorf("guilds[%d].rbac.roles is required", i)
		}
	}

	return nil
}

// GetGuild returns the configuration for a specific guild ID
func (c *Config) GetGuild(guildID string) (*GuildConfig, error) {
	for i := range c.Guilds {
		if c.Guilds[i].ID == guildID {
			return &c.Guilds[i], nil
		}
	}
	return nil, fmt.Errorf("guild %s not found in configuration", guildID)
}

// GetProvider returns a provider by name
func (c *Config) GetProvider(name string) (*Provider, error) {
	for i := range c.Providers {
		if c.Providers[i].Name == name {
			return &c.Providers[i], nil
		}
	}
	return nil, fmt.Errorf("provider %s not found", name)
}

// ResolveModel returns the provider and model for a model reference (e.g., "openai/gpt-4o")
func (c *Config) ResolveModel(modelRef string) (*Provider, *Model, error) {
	// Parse provider/model
	var providerName, modelID string
	for i, ch := range modelRef {
		if ch == '/' {
			providerName = modelRef[:i]
			modelID = modelRef[i+1:]
			break
		}
	}

	if providerName == "" || modelID == "" {
		return nil, nil, fmt.Errorf("invalid model reference: %s (expected format: provider/model)", modelRef)
	}

	// Find provider
	provider, err := c.GetProvider(providerName)
	if err != nil {
		return nil, nil, err
	}

	// Find model
	for i := range provider.Models {
		if provider.Models[i].ID == modelID {
			return provider, &provider.Models[i], nil
		}
	}

	return nil, nil, fmt.Errorf("model %s not found in provider %s", modelID, providerName)
}
