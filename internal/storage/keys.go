package storage

import (
	"fmt"
)

// Keys generates Redis keys with consistent naming
type Keys struct {
	prefix string
}

// NewKeys creates a new Keys generator
func NewKeys(prefix string) *Keys {
	return &Keys{prefix: prefix}
}

// Conversation returns the key for conversation metadata
func (k *Keys) Conversation(guildID, threadID string) string {
	return fmt.Sprintf("%s%s:conversation:%s", k.prefix, guildID, threadID)
}

// Messages returns the key for message history
func (k *Keys) Messages(guildID, threadID string) string {
	return fmt.Sprintf("%s%s:messages:%s", k.prefix, guildID, threadID)
}

// RateLimitMinute returns the key for per-minute rate limiting
func (k *Keys) RateLimitMinute(guildID, userID string) string {
	return fmt.Sprintf("%s%s:ratelimit:%s:minute", k.prefix, guildID, userID)
}

// RateLimitHour returns the key for per-hour rate limiting
func (k *Keys) RateLimitHour(guildID, userID string) string {
	return fmt.Sprintf("%s%s:ratelimit:%s:hour", k.prefix, guildID, userID)
}

// TokenLimit returns the key for token usage tracking
func (k *Keys) TokenLimit(guildID, userID string, periodStart int64) string {
	return fmt.Sprintf("%s%s:tokens:%s:%d", k.prefix, guildID, userID, periodStart)
}

// Usage returns the key for daily usage tracking
func (k *Keys) Usage(guildID, userID, date string) string {
	return fmt.Sprintf("%s%s:usage:%s:%s", k.prefix, guildID, userID, date)
}

// Prompts returns the key for guild system prompts
func (k *Keys) Prompts(guildID string) string {
	return fmt.Sprintf("%s%s:prompts", k.prefix, guildID)
}
