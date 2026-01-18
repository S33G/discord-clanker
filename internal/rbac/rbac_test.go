package rbac

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/s33g/discord-prompter/internal/config"
)

func getTestConfig() *config.Config {
	return &config.Config{
		Providers: []config.Provider{
			{
				Name:    "ollama",
				BaseURL: "http://localhost",
				Models: []config.Model{
					{ID: "llama3.2", DisplayName: "Llama 3.2"},
					{ID: "codellama", DisplayName: "Code Llama"},
				},
			},
			{
				Name:    "openai",
				BaseURL: "https://api.openai.com",
				Models: []config.Model{
					{ID: "gpt-4o", DisplayName: "GPT-4o"},
					{ID: "gpt-4o-mini", DisplayName: "GPT-4o Mini"},
				},
			},
		},
		Guilds: []config.GuildConfig{
			{
				ID: "test-guild",
				EnabledModels: []string{
					"ollama/llama3.2",
					"ollama/codellama",
					"openai/gpt-4o",
					"openai/gpt-4o-mini",
				},
				RBAC: config.RBACConfig{
					Roles: []config.RoleConfig{
						{
							DiscordRole:   "Admin",
							Permissions:   []string{"use_models", "manage_prompts", "unlimited_tokens"},
							AllowedModels: []string{"*"},
						},
						{
							DiscordRole:   "Pro",
							Permissions:   []string{"use_models"},
							AllowedModels: []string{"ollama/*", "openai/gpt-4o-mini"},
						},
						{
							DiscordRole:   "Member",
							Permissions:   []string{"use_models"},
							AllowedModels: []string{"ollama/llama3.2"},
						},
					},
				},
			},
		},
	}
}

func TestManager_HasPermission(t *testing.T) {
	cfg := getTestConfig()
	mgr := NewManager(cfg)

	tests := []struct {
		name       string
		roles      []string
		permission Permission
		want       bool
	}{
		{
			name:       "admin has manage_prompts",
			roles:      []string{"Admin"},
			permission: PermManagePrompts,
			want:       true,
		},
		{
			name:       "member doesn't have manage_prompts",
			roles:      []string{"Member"},
			permission: PermManagePrompts,
			want:       false,
		},
		{
			name:       "admin has unlimited_tokens",
			roles:      []string{"Admin"},
			permission: PermUnlimitedTokens,
			want:       true,
		},
		{
			name:       "pro member can use_models",
			roles:      []string{"Pro"},
			permission: PermUseModels,
			want:       true,
		},
		{
			name:       "no role has no permissions",
			roles:      []string{},
			permission: PermUseModels,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			member := &discordgo.Member{
				Roles: tt.roles,
			}

			got := mgr.HasPermission("test-guild", member, tt.permission)
			if got != tt.want {
				t.Errorf("HasPermission() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestManager_CanUseModel(t *testing.T) {
	cfg := getTestConfig()
	mgr := NewManager(cfg)

	tests := []struct {
		name     string
		roles    []string
		modelRef string
		want     bool
	}{
		{
			name:     "admin can use any model",
			roles:    []string{"Admin"},
			modelRef: "openai/gpt-4o",
			want:     true,
		},
		{
			name:     "pro can use ollama models",
			roles:    []string{"Pro"},
			modelRef: "ollama/llama3.2",
			want:     true,
		},
		{
			name:     "pro can use gpt-4o-mini",
			roles:    []string{"Pro"},
			modelRef: "openai/gpt-4o-mini",
			want:     true,
		},
		{
			name:     "pro cannot use gpt-4o",
			roles:    []string{"Pro"},
			modelRef: "openai/gpt-4o",
			want:     false,
		},
		{
			name:     "member can only use llama3.2",
			roles:    []string{"Member"},
			modelRef: "ollama/llama3.2",
			want:     true,
		},
		{
			name:     "member cannot use codellama",
			roles:    []string{"Member"},
			modelRef: "ollama/codellama",
			want:     false,
		},
		{
			name:     "no role cannot use any model",
			roles:    []string{},
			modelRef: "ollama/llama3.2",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			member := &discordgo.Member{
				Roles: tt.roles,
			}

			got := mgr.CanUseModel("test-guild", member, tt.modelRef)
			if got != tt.want {
				t.Errorf("CanUseModel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestManager_GetAllowedModels(t *testing.T) {
	cfg := getTestConfig()
	mgr := NewManager(cfg)

	tests := []struct {
		name      string
		roles     []string
		wantCount int
		wantHave  []string
	}{
		{
			name:      "admin gets all models",
			roles:     []string{"Admin"},
			wantCount: 4, // All enabled models
			wantHave:  []string{"ollama/llama3.2", "openai/gpt-4o"},
		},
		{
			name:      "pro gets ollama + gpt-4o-mini",
			roles:     []string{"Pro"},
			wantCount: 3,
			wantHave:  []string{"ollama/llama3.2", "ollama/codellama", "openai/gpt-4o-mini"},
		},
		{
			name:      "member gets only llama3.2",
			roles:     []string{"Member"},
			wantCount: 1,
			wantHave:  []string{"ollama/llama3.2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			member := &discordgo.Member{
				Roles: tt.roles,
			}

			got := mgr.GetAllowedModels("test-guild", member)
			if len(got) != tt.wantCount {
				t.Errorf("GetAllowedModels() count = %d, want %d", len(got), tt.wantCount)
			}

			for _, want := range tt.wantHave {
				found := false
				for _, model := range got {
					if model == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("GetAllowedModels() missing %s", want)
				}
			}
		})
	}
}

func TestMatchesModelPattern(t *testing.T) {
	tests := []struct {
		name     string
		modelRef string
		pattern  string
		want     bool
	}{
		{
			name:     "exact match",
			modelRef: "openai/gpt-4o",
			pattern:  "openai/gpt-4o",
			want:     true,
		},
		{
			name:     "wildcard all",
			modelRef: "openai/gpt-4o",
			pattern:  "*",
			want:     true,
		},
		{
			name:     "wildcard provider",
			modelRef: "openai/gpt-4o",
			pattern:  "openai/*",
			want:     true,
		},
		{
			name:     "wildcard provider no match",
			modelRef: "openai/gpt-4o",
			pattern:  "ollama/*",
			want:     false,
		},
		{
			name:     "no match",
			modelRef: "openai/gpt-4o",
			pattern:  "openai/gpt-3.5",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesModelPattern(tt.modelRef, tt.pattern)
			if got != tt.want {
				t.Errorf("matchesModelPattern(%q, %q) = %v, want %v", tt.modelRef, tt.pattern, got, tt.want)
			}
		})
	}
}
