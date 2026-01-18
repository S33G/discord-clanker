package bot

import (
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/s33g/discord-prompter/internal/config"
	"github.com/s33g/discord-prompter/internal/conversation"
	"github.com/s33g/discord-prompter/internal/llm"
)

// handleAsk handles the /ask command - creates a new conversation thread
func (b *Bot) handleAsk(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Defer initial response to avoid timeout
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	cfg := b.GetConfig()
	guildCfg, err := cfg.GetGuild(i.GuildID)
	if err != nil {
		b.editInteractionError(s, i, "This bot is not configured for this server")
		return
	}

	// Get command options
	options := i.ApplicationCommandData().Options
	prompt := getStringOption(options, "prompt")
	modelRef := getStringOption(options, "model")
	systemPromptName := getStringOption(options, "system_prompt")

	// Use defaults if not specified
	if modelRef == "" {
		modelRef = guildCfg.DefaultModel
	}

	// Get member with roles
	member, err := s.GuildMember(i.GuildID, i.Member.User.ID)
	if err != nil {
		b.editInteractionError(s, i, "Failed to get member information")
		return
	}

	// Check permissions
	if !b.rbacManager.HasPermission(i.GuildID, member, "use_models") {
		b.editInteractionError(s, i, "You don't have permission to use models")
		return
	}

	// Check if user can access this model
	if !b.rbacManager.CanUseModel(i.GuildID, member, modelRef) {
		b.editInteractionError(s, i, fmt.Sprintf("You don't have access to model: %s", modelRef))
		return
	}

	// Get rate limit config for user's role
	rateLimitCfg := b.getRateLimitForMember(guildCfg, member)

	// Check rate limits
	ctx := context.Background()
	rateResult, err := b.rateLimiter.CheckRateLimit(ctx, i.GuildID, member.User.ID, rateLimitCfg)
	if err != nil {
		b.logger.Error().Err(err).Msg("Rate limit check failed")
		b.editInteractionError(s, i, "Failed to check rate limits")
		return
	}
	if !rateResult.Allowed {
		b.editInteractionError(s, i, fmt.Sprintf("Rate limited. Try again in %d seconds.", rateResult.SecondsToReset))
		return
	}

	// Get system prompt
	systemPrompt := ""
	if systemPromptName != "" {
		systemPrompt, err = guildCfg.GetSystemPromptByName(systemPromptName)
		if err != nil {
			b.editInteractionError(s, i, fmt.Sprintf("System prompt not found: %s", systemPromptName))
			return
		}
	} else {
		systemPrompt, err = guildCfg.GetDefaultSystemPrompt()
		if err != nil {
			b.editInteractionError(s, i, "Failed to get default system prompt")
			return
		}
	}

	// Estimate tokens for the prompt
	tokenCounter := conversation.NewTokenCounter()
	promptTokens, err := tokenCounter.Count(prompt, modelRef)
	if err != nil {
		b.logger.Warn().Err(err).Msg("Failed to count tokens, using estimate")
		promptTokens = len(prompt) / 4
	}
	promptTokens += 4 // Message overhead

	systemTokens, _ := tokenCounter.Count(systemPrompt, modelRef)
	systemTokens += 4

	estimatedTokens := promptTokens + systemTokens + 1000 // Reserve for response

	// Check token limits
	tokenLimitCfg := b.getTokenLimitForMember(guildCfg, member)
	tokenResult, err := b.rateLimiter.CheckTokenLimit(ctx, i.GuildID, member.User.ID, tokenLimitCfg, estimatedTokens)
	if err != nil {
		b.logger.Error().Err(err).Msg("Token limit check failed")
		b.editInteractionError(s, i, "Failed to check token limits")
		return
	}
	if !tokenResult.Allowed {
		b.editInteractionError(s, i, fmt.Sprintf("Token limit exceeded. You have %d tokens remaining. Resets in %d seconds.", tokenResult.TokensRemaining, tokenResult.SecondsToReset))
		return
	}

	// Generate thread title using LLM
	b.logger.Info().Str("user", member.User.Username).Str("model", modelRef).Msg("Generating thread title")
	title, err := b.llmRegistry.GenerateTitle(ctx, modelRef, prompt)
	if err != nil {
		b.logger.Warn().Err(err).Msg("Failed to generate title, using fallback")
		title = prompt
		if len(title) > 80 {
			title = title[:77] + "..."
		}
	}

	// Get LLM response
	b.logger.Info().Str("user", member.User.Username).Str("model", modelRef).Msg("Calling LLM")

	messages := []conversation.Message{
		{Role: "system", Content: systemPrompt, Tokens: systemTokens},
		{Role: "user", Content: prompt, Tokens: promptTokens},
	}

	llmMessages := make([]llm.Message, len(messages))
	for i, msg := range messages {
		llmMessages[i] = llm.Message{Role: msg.Role, Content: msg.Content}
	}

	provider, _, _ := cfg.ResolveModel(modelRef)
	maxTokens := provider.DefaultMaxTokens
	if maxTokens == 0 {
		maxTokens = 2048
	}

	response, err := b.llmRegistry.Chat(ctx, modelRef, llmMessages, maxTokens, 0.7)
	if err != nil {
		b.logger.Error().Err(err).Msg("LLM request failed")
		b.editInteractionError(s, i, fmt.Sprintf("Failed to get response from %s: %v", modelRef, err))
		return
	}

	if len(response.Choices) == 0 {
		b.editInteractionError(s, i, "No response from model")
		return
	}

	assistantMessage := response.Choices[0].Message.Content

	// Create thread
	thread, err := s.MessageThreadStartComplex(i.ChannelID, i.ID, &discordgo.ThreadStart{
		Name:                title,
		AutoArchiveDuration: guildCfg.GetAutoArchiveDuration(cfg.Defaults),
		Type:                discordgo.ChannelTypeGuildPublicThread,
	})
	if err != nil {
		b.logger.Error().Err(err).Msg("Failed to create thread")
		b.editInteractionError(s, i, "Failed to create conversation thread")
		return
	}

	// Save conversation to Redis
	conv := conversation.Conversation{
		ThreadID:     thread.ID,
		GuildID:      i.GuildID,
		ChannelID:    i.ChannelID,
		UserID:       member.User.ID,
		Model:        modelRef,
		SystemPrompt: systemPrompt,
		Title:        title,
		TokenCount:   response.Usage.TotalTokens,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	ttl := guildCfg.GetConversationTTL(cfg.Defaults)
	b.convManager = conversation.NewManager(b.storage, ttl, cfg.Defaults.MessageHistoryLimit)

	if err := b.convManager.Create(ctx, conv); err != nil {
		b.logger.Error().Err(err).Msg("Failed to save conversation")
	}

	// Save messages
	b.convManager.AddMessage(ctx, i.GuildID, thread.ID, conversation.Message{
		Role:    "system",
		Content: systemPrompt,
		Tokens:  systemTokens,
	})
	b.convManager.AddMessage(ctx, i.GuildID, thread.ID, conversation.Message{
		Role:      "user",
		Content:   prompt,
		Tokens:    promptTokens,
		MessageID: i.ID,
	})

	// Post response in thread with buttons
	_, err = s.ChannelMessageSendComplex(thread.ID, &discordgo.MessageSend{
		Content: assistantMessage,
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "🔄 Regenerate",
						Style:    discordgo.PrimaryButton,
						CustomID: "regenerate",
					},
					discordgo.Button{
						Label:    "📋 Copy",
						Style:    discordgo.SecondaryButton,
						CustomID: "copy",
					},
					discordgo.Button{
						Label:    "🗑️ Clear Context",
						Style:    discordgo.DangerButton,
						CustomID: "clear",
					},
					discordgo.Button{
						Label:    "⚙️ Settings",
						Style:    discordgo.SecondaryButton,
						CustomID: "settings",
					},
				},
			},
		},
	})

	if err != nil {
		b.logger.Error().Err(err).Msg("Failed to post message in thread")
	}

	// Save assistant message
	b.convManager.AddMessage(ctx, i.GuildID, thread.ID, conversation.Message{
		Role:    "assistant",
		Content: assistantMessage,
		Tokens:  response.Usage.CompletionTokens,
	})

	// Edit original interaction to show thread link
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: stringPtr(fmt.Sprintf("✅ Created conversation: <#%s>", thread.ID)),
	})

	b.logger.Info().
		Str("user", member.User.Username).
		Str("model", modelRef).
		Str("thread", thread.ID).
		Int("tokens", response.Usage.TotalTokens).
		Msg("Conversation created")
}

// Helper functions

func getStringOption(options []*discordgo.ApplicationCommandInteractionDataOption, name string) string {
	for _, opt := range options {
		if opt.Name == name {
			return opt.StringValue()
		}
	}
	return ""
}

func (b *Bot) editInteractionError(s *discordgo.Session, i *discordgo.InteractionCreate, errMsg string) {
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: stringPtr("❌ " + errMsg),
	})
}

func stringPtr(s string) *string {
	return &s
}

func (b *Bot) getRateLimitForMember(guildCfg *config.GuildConfig, member *discordgo.Member) config.RateLimit {
	// Check role-specific limits
	for _, roleConfig := range guildCfg.RBAC.Roles {
		if contains(member.Roles, roleConfig.DiscordRole) || roleConfig.DiscordRole == "@everyone" {
			if limit, ok := guildCfg.RateLimits.Roles[roleConfig.DiscordRole]; ok {
				return limit
			}
		}
	}
	return guildCfg.RateLimits.Default
}

func (b *Bot) getTokenLimitForMember(guildCfg *config.GuildConfig, member *discordgo.Member) config.TokenLimit {
	// Check role-specific limits
	for _, roleConfig := range guildCfg.RBAC.Roles {
		if contains(member.Roles, roleConfig.DiscordRole) || roleConfig.DiscordRole == "@everyone" {
			if limit, ok := guildCfg.TokenLimits.Roles[roleConfig.DiscordRole]; ok {
				return limit
			}
		}
	}
	return guildCfg.TokenLimits.Default
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
