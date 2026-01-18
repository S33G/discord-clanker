package rbac

import (
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/s33g/discord-prompter/internal/config"
)

// Manager handles role-based access control
type Manager struct {
	config *config.Config
}

// NewManager creates a new RBAC manager
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		config: cfg,
	}
}

// HasPermission checks if a user has a specific permission
func (m *Manager) HasPermission(guildID string, member *discordgo.Member, perm Permission) bool {
	guildCfg, err := m.config.GetGuild(guildID)
	if err != nil {
		return false
	}

	// Get user's role names
	roleNames := m.getRoleNames(guildID, member)

	// Check each role config
	for _, roleConfig := range guildCfg.RBAC.Roles {
		// Check if user has this role
		if !contains(roleNames, roleConfig.DiscordRole) && roleConfig.DiscordRole != "@everyone" {
			continue
		}

		// Check if role has permission
		for _, p := range roleConfig.Permissions {
			if Permission(p) == perm {
				return true
			}
		}
	}

	return false
}

// CanUseModel checks if a user can use a specific model
func (m *Manager) CanUseModel(guildID string, member *discordgo.Member, modelRef string) bool {
	guildCfg, err := m.config.GetGuild(guildID)
	if err != nil {
		return false
	}

	// Get user's role names
	roleNames := m.getRoleNames(guildID, member)

	// Check each role config
	for _, roleConfig := range guildCfg.RBAC.Roles {
		// Check if user has this role
		if !contains(roleNames, roleConfig.DiscordRole) && roleConfig.DiscordRole != "@everyone" {
			continue
		}

		// Check if role allows this model
		for _, allowedModel := range roleConfig.AllowedModels {
			if matchesModelPattern(modelRef, allowedModel) {
				return true
			}
		}
	}

	return false
}

// GetAllowedModels returns all models a user can access
func (m *Manager) GetAllowedModels(guildID string, member *discordgo.Member) []string {
	guildCfg, err := m.config.GetGuild(guildID)
	if err != nil {
		return nil
	}

	// Get user's role names
	roleNames := m.getRoleNames(guildID, member)

	// Collect allowed models from all roles
	allowed := make(map[string]bool)
	for _, roleConfig := range guildCfg.RBAC.Roles {
		// Check if user has this role
		if !contains(roleNames, roleConfig.DiscordRole) && roleConfig.DiscordRole != "@everyone" {
			continue
		}

		// Add allowed models
		for _, pattern := range roleConfig.AllowedModels {
			if pattern == "*" {
				// User has access to all models
				return guildCfg.EnabledModels
			}

			// Check if pattern matches any enabled models
			for _, enabledModel := range guildCfg.EnabledModels {
				if matchesModelPattern(enabledModel, pattern) {
					allowed[enabledModel] = true
				}
			}
		}
	}

	// Convert to slice
	result := make([]string, 0, len(allowed))
	for model := range allowed {
		result = append(result, model)
	}

	return result
}

// getRoleNames extracts role names from a Discord member
func (m *Manager) getRoleNames(guildID string, member *discordgo.Member) []string {
	// In a real implementation, we'd fetch role names from Discord
	// For now, return role IDs (caller should ensure member has role names populated)
	names := make([]string, len(member.Roles))
	copy(names, member.Roles)
	return names
}

// matchesModelPattern checks if a model reference matches a pattern
// Supports wildcards: "provider/*" matches "provider/any-model"
func matchesModelPattern(modelRef, pattern string) bool {
	if pattern == "*" {
		return true
	}

	// Exact match
	if modelRef == pattern {
		return true
	}

	// Wildcard match (e.g., "openai/*")
	if strings.HasSuffix(pattern, "/*") {
		prefix := pattern[:len(pattern)-2]
		return strings.HasPrefix(modelRef, prefix+"/")
	}

	return false
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Reload updates the RBAC manager with new configuration
func (m *Manager) Reload(cfg *config.Config) error {
	m.config = cfg
	return nil
}
