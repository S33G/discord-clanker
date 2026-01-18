package conversation

import (
	"encoding/json"
	"fmt"
	"time"
)

// Message represents a conversation message
type Message struct {
	Role      string `json:"role"` // "system", "user", "assistant"
	Content   string `json:"content"`
	Tokens    int    `json:"tokens"`
	MessageID string `json:"msg_id,omitempty"` // Discord message ID
}

// Conversation represents conversation metadata
type Conversation struct {
	ThreadID     string
	GuildID      string
	ChannelID    string
	UserID       string
	Model        string
	SystemPrompt string
	Title        string
	TokenCount   int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ToMap converts conversation to a map for Redis HSET
func (c *Conversation) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"guild_id":      c.GuildID,
		"channel_id":    c.ChannelID,
		"user_id":       c.UserID,
		"model":         c.Model,
		"system_prompt": c.SystemPrompt,
		"title":         c.Title,
		"token_count":   c.TokenCount,
		"created_at":    c.CreatedAt.Unix(),
		"updated_at":    c.UpdatedAt.Unix(),
	}
}

// FromMap populates conversation from Redis HGETALL result
func (c *Conversation) FromMap(threadID string, m map[string]string) error {
	c.ThreadID = threadID
	c.GuildID = m["guild_id"]
	c.ChannelID = m["channel_id"]
	c.UserID = m["user_id"]
	c.Model = m["model"]
	c.SystemPrompt = m["system_prompt"]
	c.Title = m["title"]

	var tokenCount int64
	if _, err := fmt.Sscanf(m["token_count"], "%d", &tokenCount); err == nil {
		c.TokenCount = int(tokenCount)
	}

	var createdAt, updatedAt int64
	if _, err := fmt.Sscanf(m["created_at"], "%d", &createdAt); err == nil {
		c.CreatedAt = time.Unix(createdAt, 0)
	}
	if _, err := fmt.Sscanf(m["updated_at"], "%d", &updatedAt); err == nil {
		c.UpdatedAt = time.Unix(updatedAt, 0)
	}

	return nil
}

// MarshalMessage converts a Message to JSON for storage
func MarshalMessage(m Message) (string, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// UnmarshalMessage converts JSON to a Message
func UnmarshalMessage(data string) (Message, error) {
	var m Message
	err := json.Unmarshal([]byte(data), &m)
	return m, err
}
