package bot

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/s33g/discord-prompter/internal/config"
	"github.com/s33g/discord-prompter/internal/conversation"
	"github.com/s33g/discord-prompter/internal/llm"
)

// handleButton routes button interactions to specific handlers
func (b *Bot) handleButton(s *discordgo.Session, i *discordgo.InteractionCreate) {
	customID := i.MessageComponentData().CustomID

	switch customID {
	case "regenerate":
		b.handleRegenerateButton(s, i)
	case "copy":
		b.handleCopyButton(s, i)
	case "clear":
		b.handleClearButton(s, i)
	case "settings":
		b.handleSettingsButton(s, i)
	default:
		if strings.HasPrefix(customID, "model:") {
			b.handleModelSelect(s, i)
		} else if strings.HasPrefix(customID, "prompt:") {
			b.handlePromptSelect(s, i)
		} else {
			b.respondError(s, i, "Unknown button")
		}
	}
}

// handleRegenerateButton regenerates the last response
func (b *Bot) handleRegenerateButton(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Acknowledge the interaction
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})

	ctx := context.Background()
	threadID := i.ChannelID

	// Load conversation
	conv, err := b.convManager.Get(ctx, i.GuildID, threadID)
	if err != nil {
		b.logger.Error().Err(err).Msg("Failed to load conversation")
		s.ChannelMessageSend(threadID, "❌ Failed to load conversation")
		return
	}

	// Get guild config
	cfg := b.GetConfig()
	guildCfg, err := cfg.GetGuild(i.GuildID)
	if err != nil {
		b.logger.Error().Err(err).Msg("Failed to get guild config")
		return
	}

	// Get member with roles
	member, err := s.GuildMember(i.GuildID, i.Member.User.ID)
	if err != nil {
		b.logger.Error().Err(err).Msg("Failed to get member info")
		return
	}

	// Check permissions
	if !b.rbacManager.HasPermission(i.GuildID, member, "use_models") {
		s.ChannelMessageSend(threadID, "❌ You don't have permission to use models")
		return
	}

	// Get rate limit and token limit configs
	rateLimitCfg := b.getRateLimitForMember(guildCfg, member)
	tokenLimitCfg := b.getTokenLimitForMember(guildCfg, member)

	// Check rate limits
	rateResult, err := b.rateLimiter.CheckRateLimit(ctx, i.GuildID, member.User.ID, rateLimitCfg)
	if err != nil || !rateResult.Allowed {
		s.ChannelMessageSend(threadID, fmt.Sprintf("❌ Rate limited. Try again in %d seconds.", rateResult.SecondsToReset))
		return
	}

	// Load message history
	messages, err := b.convManager.GetMessages(ctx, i.GuildID, threadID)
	if err != nil {
		b.logger.Error().Err(err).Msg("Failed to load messages")
		s.ChannelMessageSend(threadID, "❌ Failed to load message history")
		return
	}

	if len(messages) == 0 {
		s.ChannelMessageSend(threadID, "❌ No message history found")
		return
	}

	// Remove the last assistant message if it exists (we'll regenerate it)
	if len(messages) > 0 && messages[len(messages)-1].Role == "assistant" {
		messages = messages[:len(messages)-1]
	}

	// Estimate tokens
	totalTokens := 0
	for _, msg := range messages {
		totalTokens += msg.Tokens
	}
	estimatedTokens := totalTokens + 1000 // Reserve for response

	// Check token limits
	tokenResult, err := b.rateLimiter.CheckTokenLimit(ctx, i.GuildID, member.User.ID, tokenLimitCfg, estimatedTokens)
	if err != nil || !tokenResult.Allowed {
		s.ChannelMessageSend(threadID, fmt.Sprintf("❌ Token limit exceeded. Resets in %d seconds.", tokenResult.SecondsToReset))
		return
	}

	// Build context
	maxContextTokens := guildCfg.GetMaxContextTokens(cfg.Defaults)
	builder := conversation.NewContextBuilder(maxContextTokens, 1000)
	contextMessages, _, err := builder.Build(messages, conv.SystemPrompt, conv.Model)
	if err != nil {
		b.logger.Error().Err(err).Msg("Failed to build context")
		s.ChannelMessageSend(threadID, "❌ Failed to build context")
		return
	}

	// Convert to LLM messages
	llmMessages := make([]llm.Message, len(contextMessages))
	for idx, msg := range contextMessages {
		llmMessages[idx] = llm.Message{Role: msg.Role, Content: msg.Content}
	}

	// Show typing
	s.ChannelTyping(threadID)

	// Get provider config
	provider, _, _ := cfg.ResolveModel(conv.Model)
	maxTokens := provider.DefaultMaxTokens
	if maxTokens == 0 {
		maxTokens = 2048
	}

	// Call LLM
	response, err := b.llmRegistry.Chat(ctx, conv.Model, llmMessages, maxTokens, 0.7)
	if err != nil {
		b.logger.Error().Err(err).Msg("LLM request failed")
		s.ChannelMessageSend(threadID, fmt.Sprintf("❌ Failed to regenerate: %v", err))
		return
	}

	if len(response.Choices) == 0 {
		s.ChannelMessageSend(threadID, "❌ No response from model")
		return
	}

	assistantContent := response.Choices[0].Message.Content

	// Post new response
	msg, err := s.ChannelMessageSendComplex(threadID, &discordgo.MessageSend{
		Content: assistantContent,
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{Label: "🔄 Regenerate", Style: discordgo.PrimaryButton, CustomID: "regenerate"},
					discordgo.Button{Label: "📋 Copy", Style: discordgo.SecondaryButton, CustomID: "copy"},
					discordgo.Button{Label: "🗑️ Clear Context", Style: discordgo.DangerButton, CustomID: "clear"},
					discordgo.Button{Label: "⚙️ Settings", Style: discordgo.SecondaryButton, CustomID: "settings"},
				},
			},
		},
	})

	if err != nil {
		b.logger.Error().Err(err).Msg("Failed to send message")
		return
	}

	// Save assistant message
	b.convManager.AddMessage(ctx, i.GuildID, threadID, conversation.Message{
		Role:      "assistant",
		Content:   assistantContent,
		Tokens:    response.Usage.CompletionTokens,
		MessageID: msg.ID,
	})

	// Update token count
	conv.TokenCount += response.Usage.TotalTokens
	b.convManager.Update(ctx, *conv)

	b.logger.Info().
		Str("user", member.User.Username).
		Str("action", "regenerate").
		Str("thread", threadID).
		Int("tokens", response.Usage.TotalTokens).
		Msg("Response regenerated")
}

// handleCopyButton sends the bot's message content as a code block
func (b *Bot) handleCopyButton(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Get the message that the button is attached to
	message := i.Message
	content := message.Content

	// Send as ephemeral message in code block
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("```\n%s\n```", content),
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	b.logger.Debug().
		Str("user", i.Member.User.Username).
		Str("action", "copy").
		Msg("Message copied")
}

// handleClearButton clears the message history while keeping conversation metadata
func (b *Bot) handleClearButton(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	threadID := i.ChannelID

	// Get member
	member, err := s.GuildMember(i.GuildID, i.Member.User.ID)
	if err != nil {
		b.respondError(s, i, "Failed to get member info")
		return
	}

	// Check permissions (only the conversation owner or admins can clear)
	conv, err := b.convManager.Get(ctx, i.GuildID, threadID)
	if err != nil {
		b.respondError(s, i, "Failed to load conversation")
		return
	}

	if conv.UserID != member.User.ID && !b.rbacManager.HasPermission(i.GuildID, member, "manage_prompts") {
		b.respondError(s, i, "You can only clear your own conversations")
		return
	}

	// Clear messages
	err = b.convManager.ClearMessages(ctx, i.GuildID, threadID)
	if err != nil {
		b.logger.Error().Err(err).Msg("Failed to clear messages")
		b.respondError(s, i, "Failed to clear conversation history")
		return
	}

	// Reset token count
	conv.TokenCount = 0
	b.convManager.Update(ctx, *conv)

	// Respond
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "✅ Conversation history cleared. Starting fresh!",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	b.logger.Info().
		Str("user", member.User.Username).
		Str("action", "clear").
		Str("thread", threadID).
		Msg("Conversation cleared")
}

// handleSettingsButton shows model and prompt selection menus
func (b *Bot) handleSettingsButton(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	threadID := i.ChannelID

	// Load conversation
	conv, err := b.convManager.Get(ctx, i.GuildID, threadID)
	if err != nil {
		b.respondError(s, i, "Failed to load conversation")
		return
	}

	// Get member
	member, err := s.GuildMember(i.GuildID, i.Member.User.ID)
	if err != nil {
		b.respondError(s, i, "Failed to get member info")
		return
	}

	// Only owner or admins can change settings
	if conv.UserID != member.User.ID && !b.rbacManager.HasPermission(i.GuildID, member, "manage_prompts") {
		b.respondError(s, i, "You can only modify your own conversations")
		return
	}

	// Get guild config
	cfg := b.GetConfig()
	guildCfg, err := cfg.GetGuild(i.GuildID)
	if err != nil {
		b.respondError(s, i, "Failed to get guild config")
		return
	}

	// Build model options (only models the user can access)
	allowedModels := b.rbacManager.GetAllowedModels(i.GuildID, member)
	modelOptions := []discordgo.SelectMenuOption{}
	for _, modelRef := range guildCfg.EnabledModels {
		// Check if user can use this model
		canUse := false
		for _, allowed := range allowedModels {
			if allowed == modelRef {
				canUse = true
				break
			}
		}
		if !canUse {
			continue
		}

		modelOptions = append(modelOptions, discordgo.SelectMenuOption{
			Label:   modelRef,
			Value:   "model:" + modelRef,
			Default: modelRef == conv.Model,
		})
	}

	// Build system prompt options
	promptOptions := []discordgo.SelectMenuOption{}
	for _, sp := range guildCfg.SystemPrompts {
		promptOptions = append(promptOptions, discordgo.SelectMenuOption{
			Label:       sp.Name,
			Value:       "prompt:" + sp.Name,
			Description: truncate(sp.Content, 100),
		})
	}

	// Build response with select menus
	components := []discordgo.MessageComponent{}

	if len(modelOptions) > 0 {
		components = append(components, discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "model_select",
					Placeholder: "Change model",
					Options:     modelOptions,
				},
			},
		})
	}

	if len(promptOptions) > 0 {
		components = append(components, discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "prompt_select",
					Placeholder: "Change system prompt",
					Options:     promptOptions,
				},
			},
		})
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    fmt.Sprintf("**Current Settings**\nModel: `%s`\nSystem Prompt: `%s`", conv.Model, findPromptName(guildCfg, conv.SystemPrompt)),
			Components: components,
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	})
}

// handleModelSelect handles model selection from settings menu
func (b *Bot) handleModelSelect(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	threadID := i.ChannelID

	data := i.MessageComponentData()
	if len(data.Values) == 0 {
		return
	}

	modelRef := strings.TrimPrefix(data.Values[0], "model:")

	// Update conversation
	conv, err := b.convManager.Get(ctx, i.GuildID, threadID)
	if err != nil {
		b.respondError(s, i, "Failed to load conversation")
		return
	}

	err = b.convManager.UpdateModel(ctx, i.GuildID, threadID, modelRef)
	if err != nil {
		b.logger.Error().Err(err).Msg("Failed to update model")
		b.respondError(s, i, "Failed to update model")
		return
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("✅ Model changed to `%s`", modelRef),
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	b.logger.Info().
		Str("user", i.Member.User.Username).
		Str("thread", threadID).
		Str("old_model", conv.Model).
		Str("new_model", modelRef).
		Msg("Model changed")
}

// handlePromptSelect handles system prompt selection from settings menu
func (b *Bot) handlePromptSelect(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	threadID := i.ChannelID

	data := i.MessageComponentData()
	if len(data.Values) == 0 {
		return
	}

	promptName := strings.TrimPrefix(data.Values[0], "prompt:")

	// Get guild config
	cfg := b.GetConfig()
	guildCfg, err := cfg.GetGuild(i.GuildID)
	if err != nil {
		b.respondError(s, i, "Failed to get guild config")
		return
	}

	// Get prompt content
	promptContent, err := guildCfg.GetSystemPromptByName(promptName)
	if err != nil {
		b.respondError(s, i, fmt.Sprintf("System prompt not found: %s", promptName))
		return
	}

	// Update conversation
	err = b.convManager.UpdateSystemPrompt(ctx, i.GuildID, threadID, promptContent)
	if err != nil {
		b.logger.Error().Err(err).Msg("Failed to update system prompt")
		b.respondError(s, i, "Failed to update system prompt")
		return
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("✅ System prompt changed to `%s`", promptName),
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	b.logger.Info().
		Str("user", i.Member.User.Username).
		Str("thread", threadID).
		Str("prompt", promptName).
		Msg("System prompt changed")
}

// Helper functions

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func findPromptName(guildCfg *config.GuildConfig, content string) string {
	for _, sp := range guildCfg.SystemPrompts {
		if sp.Content == content {
			return sp.Name
		}
	}
	return "custom"
}
