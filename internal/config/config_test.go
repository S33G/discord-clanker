package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	// Create a temporary config file
	content := `
redis:
  address: "localhost:6379"
  db: 0
  key_prefix: "test:"

providers:
  - name: test-provider
    base_url: http://localhost:8080
    api_key_env: ""
    models:
      - id: test-model
        display_name: "Test Model"
        context_window: 4096

guilds:
  - id: "123456789"
    name: "Test Guild"
    enabled_models:
      - test-provider/test-model
    default_model: test-provider/test-model
    system_prompts:
      - name: default
        content: "You are a test assistant."
        default: true
    rbac:
      roles:
        - discord_role: "Admin"
          permissions:
            - use_models
          allowed_models: ["*"]
    rate_limits:
      default:
        requests_per_minute: 10
        requests_per_hour: 100
    token_limits:
      default:
        tokens_per_period: 10000
        period_hours: 24
`

	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	// Load config
	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify loaded values
	if cfg.Redis.Address != "localhost:6379" {
		t.Errorf("Expected redis.address to be localhost:6379, got %s", cfg.Redis.Address)
	}

	if len(cfg.Providers) != 1 {
		t.Fatalf("Expected 1 provider, got %d", len(cfg.Providers))
	}

	if cfg.Providers[0].Name != "test-provider" {
		t.Errorf("Expected provider name to be test-provider, got %s", cfg.Providers[0].Name)
	}

	if len(cfg.Guilds) != 1 {
		t.Fatalf("Expected 1 guild, got %d", len(cfg.Guilds))
	}

	if cfg.Guilds[0].ID != "123456789" {
		t.Errorf("Expected guild ID to be 123456789, got %s", cfg.Guilds[0].ID)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				Redis: RedisConfig{
					Address: "localhost:6379",
				},
				Providers: []Provider{
					{
						Name:    "test",
						BaseURL: "http://localhost",
						Models: []Model{
							{ID: "model1", DisplayName: "Model 1"},
						},
					},
				},
				Guilds: []GuildConfig{
					{
						ID:            "123",
						EnabledModels: []string{"test/model1"},
						DefaultModel:  "test/model1",
						SystemPrompts: []SystemPrompt{
							{Name: "default", Content: "Test"},
						},
						RBAC: RBACConfig{
							Roles: []RoleConfig{
								{DiscordRole: "Admin", Permissions: []string{"use_models"}},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing redis address",
			config: &Config{
				Redis: RedisConfig{},
			},
			wantErr: true,
		},
		{
			name: "no providers",
			config: &Config{
				Redis: RedisConfig{Address: "localhost:6379"},
			},
			wantErr: true,
		},
		{
			name: "invalid model reference",
			config: &Config{
				Redis: RedisConfig{Address: "localhost:6379"},
				Providers: []Provider{
					{
						Name:    "test",
						BaseURL: "http://localhost",
						Models:  []Model{{ID: "model1", DisplayName: "Model 1"}},
					},
				},
				Guilds: []GuildConfig{
					{
						ID:            "123",
						EnabledModels: []string{"test/invalid"},
						DefaultModel:  "test/invalid",
						SystemPrompts: []SystemPrompt{{Name: "default", Content: "Test"}},
						RBAC:          RBACConfig{Roles: []RoleConfig{{DiscordRole: "Admin"}}},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetGuild(t *testing.T) {
	cfg := &Config{
		Guilds: []GuildConfig{
			{ID: "123", Name: "Guild 1"},
			{ID: "456", Name: "Guild 2"},
		},
	}

	guild, err := cfg.GetGuild("123")
	if err != nil {
		t.Fatalf("GetGuild() error = %v", err)
	}
	if guild.Name != "Guild 1" {
		t.Errorf("Expected Guild 1, got %s", guild.Name)
	}

	_, err = cfg.GetGuild("999")
	if err == nil {
		t.Error("Expected error for non-existent guild")
	}
}

func TestResolveModel(t *testing.T) {
	cfg := &Config{
		Providers: []Provider{
			{
				Name:    "openai",
				BaseURL: "https://api.openai.com",
				Models: []Model{
					{ID: "gpt-4o", DisplayName: "GPT-4o"},
				},
			},
		},
	}

	provider, model, err := cfg.ResolveModel("openai/gpt-4o")
	if err != nil {
		t.Fatalf("ResolveModel() error = %v", err)
	}
	if provider.Name != "openai" {
		t.Errorf("Expected provider openai, got %s", provider.Name)
	}
	if model.ID != "gpt-4o" {
		t.Errorf("Expected model gpt-4o, got %s", model.ID)
	}

	_, _, err = cfg.ResolveModel("invalid")
	if err == nil {
		t.Error("Expected error for invalid model reference")
	}
}

func TestGuildConfig_GetDefaultSystemPrompt(t *testing.T) {
	tests := []struct {
		name    string
		guild   GuildConfig
		want    string
		wantErr bool
	}{
		{
			name: "explicit default",
			guild: GuildConfig{
				DefaultSystemPrompt: "custom",
				SystemPrompts: []SystemPrompt{
					{Name: "custom", Content: "Custom prompt"},
					{Name: "other", Content: "Other prompt"},
				},
			},
			want:    "Custom prompt",
			wantErr: false,
		},
		{
			name: "default flag",
			guild: GuildConfig{
				SystemPrompts: []SystemPrompt{
					{Name: "first", Content: "First prompt"},
					{Name: "second", Content: "Second prompt", Default: true},
				},
			},
			want:    "Second prompt",
			wantErr: false,
		},
		{
			name: "first prompt fallback",
			guild: GuildConfig{
				SystemPrompts: []SystemPrompt{
					{Name: "first", Content: "First prompt"},
					{Name: "second", Content: "Second prompt"},
				},
			},
			want:    "First prompt",
			wantErr: false,
		},
		{
			name:    "no prompts",
			guild:   GuildConfig{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.guild.GetDefaultSystemPrompt()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetDefaultSystemPrompt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("GetDefaultSystemPrompt() = %v, want %v", got, tt.want)
			}
		})
	}
}
