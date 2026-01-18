package rbac

// Permission represents a permission string
type Permission string

const (
	// PermUseModels allows using LLM models
	PermUseModels Permission = "use_models"

	// PermManagePrompts allows creating/deleting system prompts
	PermManagePrompts Permission = "manage_prompts"

	// PermManageModels allows modifying model configuration
	PermManageModels Permission = "manage_models"

	// PermUnlimitedRate bypasses rate limits
	PermUnlimitedRate Permission = "unlimited_rate"

	// PermUnlimitedTokens bypasses token limits
	PermUnlimitedTokens Permission = "unlimited_tokens"

	// PermViewAllUsage allows viewing usage stats for all users
	PermViewAllUsage Permission = "view_all_usage"

	// PermReloadConfig allows hot-reloading configuration
	PermReloadConfig Permission = "reload_config"
)

// AllPermissions returns all defined permissions
func AllPermissions() []Permission {
	return []Permission{
		PermUseModels,
		PermManagePrompts,
		PermManageModels,
		PermUnlimitedRate,
		PermUnlimitedTokens,
		PermViewAllUsage,
		PermReloadConfig,
	}
}

// IsValid checks if a permission string is valid
func (p Permission) IsValid() bool {
	for _, valid := range AllPermissions() {
		if p == valid {
			return true
		}
	}
	return false
}
