package bot

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/s33g/discord-prompter/internal/config"
)

// handleModels shows available models for the user
func (b *Bot) handleModels(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Get member with roles
	member, err := s.GuildMember(i.GuildID, i.Member.User.ID)
	if err != nil {
		b.respondError(s, i, "Failed to get member information")
		return
	}

	// Get allowed models for this user
	allowedModels := b.rbacManager.GetAllowedModels(i.GuildID, member)

	if len(allowedModels) == 0 {
		b.respondMessage(s, i, "❌ You don't have access to any models")
		return
	}

	// Get guild config to show model details
	cfg := b.GetConfig()
	guildCfg, err := cfg.GetGuild(i.GuildID)
	if err != nil {
		b.respondError(s, i, "Failed to get guild configuration")
		return
	}

	// Build response
	var sb strings.Builder
	sb.WriteString("**Available Models**\n\n")

	for _, modelRef := range allowedModels {
		// Check if model is enabled
		enabled := false
		for _, enabledModel := range guildCfg.EnabledModels {
			if enabledModel == modelRef {
				enabled = true
				break
			}
		}

		if !enabled {
			continue
		}

		provider, model, err := cfg.ResolveModel(modelRef)
		if err != nil {
			sb.WriteString(fmt.Sprintf("• `%s` - *Unknown model*\n", modelRef))
			continue
		}

		isDefault := ""
		if modelRef == guildCfg.DefaultModel {
			isDefault = " *(default)*"
		}

		sb.WriteString(fmt.Sprintf("• `%s`%s\n", modelRef, isDefault))
		if model.DisplayName != "" {
			sb.WriteString(fmt.Sprintf("  - Name: %s\n", model.DisplayName))
		}
		if model.ContextWindow > 0 {
			sb.WriteString(fmt.Sprintf("  - Context: %d tokens\n", model.ContextWindow))
		}
		sb.WriteString(fmt.Sprintf("  - Provider: %s\n", provider.Name))
		sb.WriteString("\n")
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: sb.String(),
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	b.logger.Info().
		Str("user", member.User.Username).
		Str("command", "models").
		Int("models_shown", len(allowedModels)).
		Msg("Models listed")
}

// handlePrompts shows available system prompts
func (b *Bot) handlePrompts(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Get guild config
	cfg := b.GetConfig()
	guildCfg, err := cfg.GetGuild(i.GuildID)
	if err != nil {
		b.respondError(s, i, "Failed to get guild configuration")
		return
	}

	if len(guildCfg.SystemPrompts) == 0 {
		b.respondMessage(s, i, "❌ No system prompts configured")
		return
	}

	// Build response
	var sb strings.Builder
	sb.WriteString("**Available System Prompts**\n\n")

	for _, sp := range guildCfg.SystemPrompts {
		isDefault := ""
		if sp.Default || sp.Name == guildCfg.DefaultSystemPrompt {
			isDefault = " *(default)*"
		}

		sb.WriteString(fmt.Sprintf("• **%s**%s\n", sp.Name, isDefault))

		// Show preview of content
		preview := sp.Content
		if len(preview) > 200 {
			preview = preview[:197] + "..."
		}
		sb.WriteString(fmt.Sprintf("  ```\n  %s\n  ```\n\n", preview))
	}

	// Split message if too long (Discord limit is 2000 chars)
	content := sb.String()
	if len(content) > 1900 {
		// Send simplified version
		sb.Reset()
		sb.WriteString("**Available System Prompts**\n\n")
		for _, sp := range guildCfg.SystemPrompts {
			isDefault := ""
			if sp.Default || sp.Name == guildCfg.DefaultSystemPrompt {
				isDefault = " *(default)*"
			}
			sb.WriteString(fmt.Sprintf("• `%s`%s\n", sp.Name, isDefault))
		}
		content = sb.String()
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	member, _ := s.GuildMember(i.GuildID, i.Member.User.ID)
	b.logger.Info().
		Str("user", member.User.Username).
		Str("command", "prompts").
		Int("prompts_shown", len(guildCfg.SystemPrompts)).
		Msg("Prompts listed")
}

// handleUsage shows usage statistics for the user
func (b *Bot) handleUsage(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()

	// Get member
	member, err := s.GuildMember(i.GuildID, i.Member.User.ID)
	if err != nil {
		b.respondError(s, i, "Failed to get member information")
		return
	}

	// Get guild config
	cfg := b.GetConfig()
	guildCfg, err := cfg.GetGuild(i.GuildID)
	if err != nil {
		b.respondError(s, i, "Failed to get guild configuration")
		return
	}

	// Get rate limit status
	rateLimitCfg := b.getRateLimitForMember(guildCfg, member)
	rateStatus, err := b.rateLimiter.CheckRateLimit(ctx, i.GuildID, member.User.ID, rateLimitCfg)
	if err != nil {
		b.logger.Error().Err(err).Msg("Failed to check rate limit")
	}

	// Get token limit status
	tokenLimitCfg := b.getTokenLimitForMember(guildCfg, member)
	tokenStatus, err := b.rateLimiter.CheckTokenLimit(ctx, i.GuildID, member.User.ID, tokenLimitCfg, 0)
	if err != nil {
		b.logger.Error().Err(err).Msg("Failed to check token limit")
	}

	// Build response
	var sb strings.Builder
	sb.WriteString("**Your Usage Statistics**\n\n")

	// Rate limits
	sb.WriteString("**Rate Limits**\n")
	if rateLimitCfg.RequestsPerMinute > 0 {
		status := "Unknown"
		if rateStatus != nil && rateStatus.Allowed {
			status = "Available"
		} else if rateStatus != nil && !rateStatus.Allowed {
			status = fmt.Sprintf("Limited (reset in %ds)", rateStatus.SecondsToReset)
		}
		sb.WriteString(fmt.Sprintf("• Per Minute: %d limit - %s\n", rateLimitCfg.RequestsPerMinute, status))
	}
	if rateLimitCfg.RequestsPerHour > 0 {
		sb.WriteString(fmt.Sprintf("• Per Hour: %d limit\n", rateLimitCfg.RequestsPerHour))
	}
	if rateLimitCfg.RequestsPerMinute == 0 && rateLimitCfg.RequestsPerHour == 0 {
		sb.WriteString("• Unlimited\n")
	}
	sb.WriteString("\n")

	// Token limits
	sb.WriteString("**Token Limits**\n")
	if tokenLimitCfg.Bypass {
		sb.WriteString("• Unlimited (bypass enabled)\n")
	} else if tokenLimitCfg.TokensPerPeriod > 0 {
		remaining := tokenLimitCfg.TokensPerPeriod
		if tokenStatus != nil {
			remaining = tokenStatus.TokensRemaining
		}
		sb.WriteString(fmt.Sprintf("• Per %d hours: %d / %d tokens remaining\n",
			tokenLimitCfg.PeriodHours, remaining, tokenLimitCfg.TokensPerPeriod))
		if tokenStatus != nil && tokenStatus.SecondsToReset > 0 {
			hours := tokenStatus.SecondsToReset / 3600
			minutes := (tokenStatus.SecondsToReset % 3600) / 60
			sb.WriteString(fmt.Sprintf("• Resets in: %dh %dm\n", hours, minutes))
		}
	} else {
		sb.WriteString("• No token limits configured\n")
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: sb.String(),
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	b.logger.Info().
		Str("user", member.User.Username).
		Str("command", "usage").
		Msg("Usage displayed")
}

// handleReload reloads the configuration
func (b *Bot) handleReload(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Get member
	member, err := s.GuildMember(i.GuildID, i.Member.User.ID)
	if err != nil {
		b.respondError(s, i, "Failed to get member information")
		return
	}

	// Check permission
	if !b.rbacManager.HasPermission(i.GuildID, member, "reload_config") {
		b.respondError(s, i, "You don't have permission to reload the configuration")
		return
	}

	// Respond immediately
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "🔄 Reloading configuration...",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	// Reload config from file
	newCfg, err := config.Load(b.configPath)
	if err != nil {
		b.logger.Error().Err(err).Msg("Failed to reload config")
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: stringPtr(fmt.Sprintf("❌ Failed to reload configuration: %v", err)),
		})
		return
	}

	// Apply new config
	err = b.Reload(newCfg)
	if err != nil {
		b.logger.Error().Err(err).Msg("Failed to apply config")
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: stringPtr(fmt.Sprintf("❌ Failed to apply configuration: %v", err)),
		})
		return
	}

	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: stringPtr("✅ Configuration reloaded successfully"),
	})

	b.logger.Info().
		Str("user", member.User.Username).
		Str("command", "reload").
		Msg("Configuration reloaded")
}
