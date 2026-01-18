package conversation

import (
	"context"
	"fmt"
	"time"

	"github.com/s33g/discord-prompter/internal/storage"
)

// Manager handles conversation storage and retrieval
type Manager struct {
	client      *storage.Client
	ttl         time.Duration
	maxMessages int
}

// NewManager creates a new conversation manager
func NewManager(client *storage.Client, ttl time.Duration, maxMessages int) *Manager {
	return &Manager{
		client:      client,
		ttl:         ttl,
		maxMessages: maxMessages,
	}
}

// Create creates a new conversation
func (m *Manager) Create(ctx context.Context, conv Conversation) error {
	now := time.Now()
	conv.CreatedAt = now
	conv.UpdatedAt = now

	key := m.client.Keys().Conversation(conv.GuildID, conv.ThreadID)

	// Store conversation metadata
	if err := m.client.Redis().HSet(ctx, key, conv.ToMap()).Err(); err != nil {
		return fmt.Errorf("failed to create conversation: %w", err)
	}

	// Set TTL
	if err := m.client.Redis().Expire(ctx, key, m.ttl).Err(); err != nil {
		return fmt.Errorf("failed to set conversation TTL: %w", err)
	}

	// Create empty message list with TTL
	msgKey := m.client.Keys().Messages(conv.GuildID, conv.ThreadID)
	if err := m.client.Redis().Expire(ctx, msgKey, m.ttl).Err(); err != nil {
		return fmt.Errorf("failed to set messages TTL: %w", err)
	}

	return nil
}

// Get retrieves a conversation by thread ID
func (m *Manager) Get(ctx context.Context, guildID, threadID string) (*Conversation, error) {
	key := m.client.Keys().Conversation(guildID, threadID)

	data, err := m.client.Redis().HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("conversation not found")
	}

	var conv Conversation
	if err := conv.FromMap(threadID, data); err != nil {
		return nil, err
	}

	return &conv, nil
}

// Update updates conversation metadata
func (m *Manager) Update(ctx context.Context, conv Conversation) error {
	conv.UpdatedAt = time.Now()

	key := m.client.Keys().Conversation(conv.GuildID, conv.ThreadID)

	if err := m.client.Redis().HSet(ctx, key, conv.ToMap()).Err(); err != nil {
		return fmt.Errorf("failed to update conversation: %w", err)
	}

	return nil
}

// Delete deletes a conversation and its messages
func (m *Manager) Delete(ctx context.Context, guildID, threadID string) error {
	convKey := m.client.Keys().Conversation(guildID, threadID)
	msgKey := m.client.Keys().Messages(guildID, threadID)

	pipe := m.client.Redis().Pipeline()
	pipe.Del(ctx, convKey)
	pipe.Del(ctx, msgKey)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}

	return nil
}

// AddMessage adds a message to the conversation history
func (m *Manager) AddMessage(ctx context.Context, guildID, threadID string, msg Message) error {
	msgKey := m.client.Keys().Messages(guildID, threadID)

	// Marshal message
	data, err := MarshalMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	pipe := m.client.Redis().Pipeline()

	// Append message
	pipe.RPush(ctx, msgKey, data)

	// Trim to max size
	pipe.LTrim(ctx, msgKey, -int64(m.maxMessages), -1)

	// Update TTL
	pipe.Expire(ctx, msgKey, m.ttl)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to add message: %w", err)
	}

	return nil
}

// GetMessages retrieves all messages in a conversation
func (m *Manager) GetMessages(ctx context.Context, guildID, threadID string) ([]Message, error) {
	msgKey := m.client.Keys().Messages(guildID, threadID)

	data, err := m.client.Redis().LRange(ctx, msgKey, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	messages := make([]Message, 0, len(data))
	for _, d := range data {
		msg, err := UnmarshalMessage(d)
		if err != nil {
			// Skip malformed messages
			continue
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// ClearMessages removes all messages from a conversation (keeps conversation metadata)
func (m *Manager) ClearMessages(ctx context.Context, guildID, threadID string) error {
	msgKey := m.client.Keys().Messages(guildID, threadID)

	if err := m.client.Redis().Del(ctx, msgKey).Err(); err != nil {
		return fmt.Errorf("failed to clear messages: %w", err)
	}

	return nil
}

// UpdateModel changes the model for a conversation
func (m *Manager) UpdateModel(ctx context.Context, guildID, threadID, model string) error {
	key := m.client.Keys().Conversation(guildID, threadID)

	pipe := m.client.Redis().Pipeline()
	pipe.HSet(ctx, key, "model", model)
	pipe.HSet(ctx, key, "updated_at", time.Now().Unix())

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to update model: %w", err)
	}

	return nil
}

// UpdateSystemPrompt changes the system prompt for a conversation
func (m *Manager) UpdateSystemPrompt(ctx context.Context, guildID, threadID, prompt string) error {
	key := m.client.Keys().Conversation(guildID, threadID)

	pipe := m.client.Redis().Pipeline()
	pipe.HSet(ctx, key, "system_prompt", prompt)
	pipe.HSet(ctx, key, "updated_at", time.Now().Unix())

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to update system prompt: %w", err)
	}

	return nil
}

// UpdateTitle updates the conversation title
func (m *Manager) UpdateTitle(ctx context.Context, guildID, threadID, title string) error {
	key := m.client.Keys().Conversation(guildID, threadID)

	pipe := m.client.Redis().Pipeline()
	pipe.HSet(ctx, key, "title", title)
	pipe.HSet(ctx, key, "updated_at", time.Now().Unix())

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to update title: %w", err)
	}

	return nil
}

// IncrementTokenCount adds tokens to the conversation's total
func (m *Manager) IncrementTokenCount(ctx context.Context, guildID, threadID string, tokens int) error {
	key := m.client.Keys().Conversation(guildID, threadID)

	if err := m.client.Redis().HIncrBy(ctx, key, "token_count", int64(tokens)).Err(); err != nil {
		return fmt.Errorf("failed to increment token count: %w", err)
	}

	return nil
}
