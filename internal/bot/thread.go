package bot

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/s33g/discord-prompter/internal/conversation"
	"github.com/s33g/discord-prompter/internal/llm"
)

// handleThreadMessage handles messages in conversation threads
func (b *Bot) handleThreadMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	ctx := context.Background()

	// Load conversation metadata
	conv, err := b.convManager.Get(ctx, m.GuildID, m.ChannelID)
	if err != nil {
		b.logger.Debug().Err(err).Str("thread", m.ChannelID).Msg("Not a tracked conversation thread")
		return
	}

	// Get guild config
	cfg := b.GetConfig()
	guildCfg, err := cfg.GetGuild(m.GuildID)
	if err != nil {
		b.logger.Error().Err(err).Str("guild", m.GuildID).Msg("Failed to get guild config")
		return
	}

	// Get member with roles
	member, err := s.GuildMember(m.GuildID, m.Author.ID)
	if err != nil {
		b.logger.Error().Err(err).Str("user", m.Author.ID).Msg("Failed to get member info")
		return
	}

	// Check permissions
	if !b.rbacManager.HasPermission(m.GuildID, member, "use_models") {
		s.ChannelMessageSend(m.ChannelID, "❌ You don't have permission to use models")
		return
	}

	// Get rate limit config
	rateLimitCfg := b.getRateLimitForMember(guildCfg, member)

	// Check rate limits
	rateResult, err := b.rateLimiter.CheckRateLimit(ctx, m.GuildID, m.Author.ID, rateLimitCfg)
	if err != nil {
		b.logger.Error().Err(err).Msg("Rate limit check failed")
		s.ChannelMessageSend(m.ChannelID, "❌ Failed to check rate limits")
		return
	}
	if !rateResult.Allowed {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("❌ Rate limited. Try again in %d seconds.", rateResult.SecondsToReset))
		return
	}

	// Count tokens in the new message
	tokenCounter := conversation.NewTokenCounter()
	userTokens, err := tokenCounter.Count(m.Content, conv.Model)
	if err != nil {
		b.logger.Warn().Err(err).Msg("Failed to count tokens, using estimate")
		userTokens = len(m.Content) / 4
	}
	userTokens += 4 // Message overhead

	// Get token limit config
	tokenLimitCfg := b.getTokenLimitForMember(guildCfg, member)

	// Estimate tokens for the response (reserve 1000)
	estimatedTokens := userTokens + 1000

	// Check token limits
	tokenResult, err := b.rateLimiter.CheckTokenLimit(ctx, m.GuildID, m.Author.ID, tokenLimitCfg, estimatedTokens)
	if err != nil {
		b.logger.Error().Err(err).Msg("Token limit check failed")
		s.ChannelMessageSend(m.ChannelID, "❌ Failed to check token limits")
		return
	}
	if !tokenResult.Allowed {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("❌ Token limit exceeded. You have %d tokens remaining. Resets in %d seconds.", tokenResult.TokensRemaining, tokenResult.SecondsToReset))
		return
	}

	// Show typing indicator
	s.ChannelTyping(m.ChannelID)

	// Load message history
	messages, err := b.convManager.GetMessages(ctx, m.GuildID, m.ChannelID)
	if err != nil {
		b.logger.Error().Err(err).Msg("Failed to load message history")
		s.ChannelMessageSend(m.ChannelID, "❌ Failed to load conversation history")
		return
	}

	// Add the new user message to the history
	newUserMsg := conversation.Message{
		Role:      "user",
		Content:   m.Content,
		Tokens:    userTokens,
		MessageID: m.ID,
	}
	messages = append(messages, newUserMsg)

	// Build context within token limits
	maxContextTokens := guildCfg.GetMaxContextTokens(cfg.Defaults)
	reserveTokens := 1000 // Reserve for response
	builder := conversation.NewContextBuilder(maxContextTokens, reserveTokens)
	contextMessages, totalContextTokens, err := builder.Build(messages, conv.SystemPrompt, conv.Model)
	if err != nil {
		b.logger.Error().Err(err).Msg("Failed to build context")
		s.ChannelMessageSend(m.ChannelID, "❌ Failed to build conversation context")
		return
	}

	// Convert to LLM messages
	llmMessages := make([]llm.Message, len(contextMessages))
	for i, msg := range contextMessages {
		llmMessages[i] = llm.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Get provider config for max tokens
	provider, _, _ := cfg.ResolveModel(conv.Model)
	maxTokens := provider.DefaultMaxTokens
	if maxTokens == 0 {
		maxTokens = 2048
	}

	// Call LLM
	b.logger.Info().
		Str("user", m.Author.Username).
		Str("model", conv.Model).
		Str("thread", m.ChannelID).
		Int("context_messages", len(contextMessages)).
		Int("context_tokens", totalContextTokens).
		Msg("Calling LLM")

	response, err := b.llmRegistry.Chat(ctx, conv.Model, llmMessages, maxTokens, 0.7)
	if err != nil {
		b.logger.Error().Err(err).Msg("LLM request failed")
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("❌ Failed to get response from %s: %v", conv.Model, err))
		return
	}

	if len(response.Choices) == 0 {
		s.ChannelMessageSend(m.ChannelID, "❌ No response from model")
		return
	}

	assistantContent := response.Choices[0].Message.Content

	// Post response with buttons
	msg, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Content: assistantContent,
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
		b.logger.Error().Err(err).Msg("Failed to send message")
		return
	}

	// Save both user and assistant messages
	b.convManager.AddMessage(ctx, m.GuildID, m.ChannelID, newUserMsg)
	b.convManager.AddMessage(ctx, m.GuildID, m.ChannelID, conversation.Message{
		Role:      "assistant",
		Content:   assistantContent,
		Tokens:    response.Usage.CompletionTokens,
		MessageID: msg.ID,
	})

	// Update conversation token count
	conv.TokenCount += response.Usage.TotalTokens
	b.convManager.Update(ctx, *conv)

	b.logger.Info().
		Str("user", m.Author.Username).
		Str("model", conv.Model).
		Str("thread", m.ChannelID).
		Int("tokens", response.Usage.TotalTokens).
		Int("total_tokens", conv.TokenCount).
		Msg("Response sent")
}
