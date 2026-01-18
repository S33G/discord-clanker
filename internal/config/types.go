package config

import (
	"fmt"
	"time"
)

// Config represents the complete application configuration
type Config struct {
	Discord   DiscordConfig  `yaml:"discord"`
	Redis     RedisConfig    `yaml:"redis"`
	Defaults  DefaultsConfig `yaml:"defaults"`
	Providers []Provider     `yaml:"providers"`
	Guilds    []GuildConfig  `yaml:"guilds"`
	Logging   LoggingConfig  `yaml:"logging"`
}

// DiscordConfig holds Discord bot settings
type DiscordConfig struct {
	Token   string `yaml:"-"` // From environment, not YAML
	GuildID string `yaml:"guild_id,omitempty"`
}

// RedisConfig holds Redis connection settings
type RedisConfig struct {
	Address     string `yaml:"address"`
	PasswordEnv string `yaml:"password_env"`
	DB          int    `yaml:"db"`
	KeyPrefix   string `yaml:"key_prefix"`
}

// DefaultsConfig holds default values applied to all guilds
type DefaultsConfig struct {
	MaxContextTokens         int `yaml:"max_context_tokens"`
	ConversationTTLHours     int `yaml:"conversation_ttl_hours"`
	UsageRetentionDays       int `yaml:"usage_retention_days"`
	MessageHistoryLimit      int `yaml:"message_history_limit"`
	ThreadAutoArchiveMinutes int `yaml:"thread_auto_archive_minutes"`
}

// ConversationTTL returns the conversation TTL as a Duration
func (d *DefaultsConfig) ConversationTTL() time.Duration {
	return time.Duration(d.ConversationTTLHours) * time.Hour
}

// Provider represents an LLM provider configuration
type Provider struct {
	Name             string  `yaml:"name"`
	BaseURL          string  `yaml:"base_url"`
	APIKeyEnv        string  `yaml:"api_key_env"`
	DefaultMaxTokens int     `yaml:"default_max_tokens"`
	Models           []Model `yaml:"models"`
}

// Model represents an LLM model configuration
type Model struct {
	ID            string `yaml:"id"`
	DisplayName   string `yaml:"display_name"`
	ContextWindow int    `yaml:"context_window"`
}

// GuildConfig holds per-guild configuration
type GuildConfig struct {
	ID                   string            `yaml:"id"`
	Name                 string            `yaml:"name"`
	EnabledModels        []string          `yaml:"enabled_models"`
	DefaultModel         string            `yaml:"default_model"`
	DefaultSystemPrompt  string            `yaml:"default_system_prompt"`
	MaxContextTokens     *int              `yaml:"max_context_tokens,omitempty"`
	ConversationTTLHours *int              `yaml:"conversation_ttl_hours,omitempty"`
	UsageRetentionDays   *int              `yaml:"usage_retention_days,omitempty"`
	SystemPrompts        []SystemPrompt    `yaml:"system_prompts"`
	RBAC                 RBACConfig        `yaml:"rbac"`
	RateLimits           RateLimitsConfig  `yaml:"rate_limits"`
	TokenLimits          TokenLimitsConfig `yaml:"token_limits"`
}

// SystemPrompt represents a system prompt template
type SystemPrompt struct {
	Name    string `yaml:"name"`
	Content string `yaml:"content"`
	Default bool   `yaml:"default,omitempty"`
}

// RBACConfig holds role-based access control settings
type RBACConfig struct {
	Roles []RoleConfig `yaml:"roles"`
}

// RoleConfig defines permissions for a Discord role
type RoleConfig struct {
	DiscordRole   string   `yaml:"discord_role"`
	Permissions   []string `yaml:"permissions"`
	AllowedModels []string `yaml:"allowed_models"`
}

// RateLimitsConfig holds rate limiting settings
type RateLimitsConfig struct {
	Default RateLimit            `yaml:"default"`
	Roles   map[string]RateLimit `yaml:"roles,omitempty"`
}

// RateLimit defines rate limiting parameters
type RateLimit struct {
	RequestsPerMinute int `yaml:"requests_per_minute"`
	RequestsPerHour   int `yaml:"requests_per_hour"`
}

// TokenLimitsConfig holds token limiting settings
type TokenLimitsConfig struct {
	Default TokenLimit            `yaml:"default"`
	Roles   map[string]TokenLimit `yaml:"roles,omitempty"`
}

// TokenLimit defines token usage limits
type TokenLimit struct {
	Bypass          bool `yaml:"bypass,omitempty"`
	TokensPerPeriod int  `yaml:"tokens_per_period,omitempty"`
	PeriodHours     int  `yaml:"period_hours,omitempty"`
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// GetMaxContextTokens returns the max context tokens for this guild (with fallback to defaults)
func (g *GuildConfig) GetMaxContextTokens(defaults DefaultsConfig) int {
	if g.MaxContextTokens != nil {
		return *g.MaxContextTokens
	}
	return defaults.MaxContextTokens
}

// GetConversationTTL returns the conversation TTL duration for this guild
func (g *GuildConfig) GetConversationTTL(defaults DefaultsConfig) time.Duration {
	hours := defaults.ConversationTTLHours
	if g.ConversationTTLHours != nil {
		hours = *g.ConversationTTLHours
	}
	return time.Duration(hours) * time.Hour
}

// GetUsageRetentionDays returns the usage retention days for this guild
func (g *GuildConfig) GetUsageRetentionDays(defaults DefaultsConfig) int {
	if g.UsageRetentionDays != nil {
		return *g.UsageRetentionDays
	}
	return defaults.UsageRetentionDays
}

// GetDefaultSystemPrompt returns the default system prompt for this guild
func (g *GuildConfig) GetDefaultSystemPrompt() (string, error) {
	// Use explicit default if set
	if g.DefaultSystemPrompt != "" {
		for _, sp := range g.SystemPrompts {
			if sp.Name == g.DefaultSystemPrompt {
				return sp.Content, nil
			}
		}
		return "", fmt.Errorf("default system prompt '%s' not found", g.DefaultSystemPrompt)
	}

	// Find first prompt marked as default
	for _, sp := range g.SystemPrompts {
		if sp.Default {
			return sp.Content, nil
		}
	}

	// Use first prompt if none marked as default
	if len(g.SystemPrompts) > 0 {
		return g.SystemPrompts[0].Content, nil
	}

	return "", fmt.Errorf("no system prompts configured")
}

// GetSystemPromptByName returns a system prompt by name
func (g *GuildConfig) GetSystemPromptByName(name string) (string, error) {
	for _, sp := range g.SystemPrompts {
		if sp.Name == name {
			return sp.Content, nil
		}
	}
	return "", fmt.Errorf("system prompt '%s' not found", name)
}

// GetAutoArchiveDuration returns the thread auto archive duration in minutes
func (g *GuildConfig) GetAutoArchiveDuration(defaults DefaultsConfig) int {
	return defaults.ThreadAutoArchiveMinutes
}
