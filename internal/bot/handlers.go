package bot

import (
	"github.com/bwmarrin/discordgo"
)

// handleInteractionCreate handles slash command and button interactions
func (b *Bot) handleInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		b.handleCommand(s, i)
	case discordgo.InteractionMessageComponent:
		b.handleButton(s, i)
	}
}

// handleCommand routes slash commands to their handlers
func (b *Bot) handleCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	cmdName := i.ApplicationCommandData().Name

	switch cmdName {
	case "ask":
		b.handleAsk(s, i)
	case "models":
		b.handleModels(s, i)
	case "prompts":
		b.handlePrompts(s, i)
	case "usage":
		b.handleUsage(s, i)
	case "reload":
		b.handleReload(s, i)
	default:
		b.respondError(s, i, "Unknown command")
	}
}

// handleMessageCreate handles messages in threads
func (b *Bot) handleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore bot messages
	if m.Author.Bot {
		return
	}

	// Check if message is in a thread
	channel, err := s.Channel(m.ChannelID)
	if err != nil || !channel.IsThread() {
		return
	}

	// Handle thread conversation
	b.handleThreadMessage(s, m)
}

// Helper functions
func (b *Bot) respondMessage(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
		},
	})
}

func (b *Bot) respondError(s *discordgo.Session, i *discordgo.InteractionCreate, errMsg string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "❌ " + errMsg,
			Flags:   discordgo.MessageFlagsEphemeral, // Only visible to user
		},
	})
}
