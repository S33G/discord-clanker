package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/s33g/discord-prompter/internal/config"
	"github.com/s33g/discord-prompter/internal/storage"
)

const rateLimitScript = `
local minute = tonumber(redis.call('GET', KEYS[1]) or "0")
local hour = tonumber(redis.call('GET', KEYS[2]) or "0")
local minute_limit = tonumber(ARGV[1])
local hour_limit = tonumber(ARGV[2])

if minute_limit > 0 and minute >= minute_limit then
    local ttl = redis.call('TTL', KEYS[1])
    return {-1, ttl > 0 and ttl or 60}
end

if hour_limit > 0 and hour >= hour_limit then
    local ttl = redis.call('TTL', KEYS[2])
    return {-2, ttl > 0 and ttl or 3600}
end

if minute == 0 then
    redis.call('SET', KEYS[1], 1, 'EX', tonumber(ARGV[3]))
else
    redis.call('INCR', KEYS[1])
end

if hour == 0 then
    redis.call('SET', KEYS[2], 1, 'EX', tonumber(ARGV[4]))
else
    redis.call('INCR', KEYS[2])
end

return {1, 0}
`

const tokenLimitScript = `
local limit = tonumber(ARGV[1])

if limit == 0 then
    return {1, 0, 0, 0}
end

local used = tonumber(redis.call('GET', KEYS[1]) or "0")
local to_add = tonumber(ARGV[3])

if used + to_add > limit then
    local ttl = redis.call('TTL', KEYS[1])
    return {-1, used, limit - used, ttl > 0 and ttl or tonumber(ARGV[2])}
end

if used == 0 then
    redis.call('SET', KEYS[1], to_add, 'EX', tonumber(ARGV[2]))
else
    redis.call('INCRBY', KEYS[1], to_add)
end

used = used + to_add
local remaining = limit - used

return {1, used, remaining, 0}
`

// Limiter handles rate limiting and token limiting
type Limiter struct {
	client        *storage.Client
	rateLimitSHA  string
	tokenLimitSHA string
}

// NewLimiter creates a new rate limiter
func NewLimiter(client *storage.Client) (*Limiter, error) {
	ctx := context.Background()

	// Load Lua scripts
	rateSHA, err := client.Redis().ScriptLoad(ctx, rateLimitScript).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to load rate limit script: %w", err)
	}

	tokenSHA, err := client.Redis().ScriptLoad(ctx, tokenLimitScript).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to load token limit script: %w", err)
	}

	return &Limiter{
		client:        client,
		rateLimitSHA:  rateSHA,
		tokenLimitSHA: tokenSHA,
	}, nil
}

// RateLimitResult holds the result of a rate limit check
type RateLimitResult struct {
	Allowed        bool
	SecondsToReset int
	LimitType      string // "minute" or "hour"
}

// CheckRateLimit checks and increments rate limits
func (l *Limiter) CheckRateLimit(ctx context.Context, guildID, userID string, limits config.RateLimit) (*RateLimitResult, error) {
	minuteKey := l.client.Keys().RateLimitMinute(guildID, userID)
	hourKey := l.client.Keys().RateLimitHour(guildID, userID)

	// Execute Lua script
	result, err := l.client.Redis().EvalSha(ctx, l.rateLimitSHA, []string{minuteKey, hourKey},
		limits.RequestsPerMinute,
		limits.RequestsPerHour,
		60,   // minute TTL
		3600, // hour TTL
	).Result()

	if err != nil {
		return nil, fmt.Errorf("rate limit check failed: %w", err)
	}

	// Parse result
	values, ok := result.([]interface{})
	if !ok || len(values) != 2 {
		return nil, fmt.Errorf("unexpected rate limit result format")
	}

	status, _ := values[0].(int64)
	seconds, _ := values[1].(int64)

	switch status {
	case 1:
		return &RateLimitResult{Allowed: true}, nil
	case -1:
		return &RateLimitResult{
			Allowed:        false,
			SecondsToReset: int(seconds),
			LimitType:      "minute",
		}, nil
	case -2:
		return &RateLimitResult{
			Allowed:        false,
			SecondsToReset: int(seconds),
			LimitType:      "hour",
		}, nil
	default:
		return nil, fmt.Errorf("unknown rate limit status: %d", status)
	}
}

// TokenLimitResult holds the result of a token limit check
type TokenLimitResult struct {
	Allowed         bool
	TokensUsed      int
	TokensRemaining int
	SecondsToReset  int
}

// CheckTokenLimit checks and increments token usage
func (l *Limiter) CheckTokenLimit(ctx context.Context, guildID, userID string, limit config.TokenLimit, tokensToAdd int) (*TokenLimitResult, error) {
	// Handle bypass mode
	if limit.Bypass {
		return &TokenLimitResult{
			Allowed:         true,
			TokensUsed:      0,
			TokensRemaining: 0,
			SecondsToReset:  0,
		}, nil
	}

	// Calculate period start timestamp
	now := time.Now()
	periodSeconds := int64(limit.PeriodHours * 3600)
	periodStart := (now.Unix() / periodSeconds) * periodSeconds

	key := l.client.Keys().TokenLimit(guildID, userID, periodStart)

	tokenLimit := limit.TokensPerPeriod

	// Execute Lua script
	result, err := l.client.Redis().EvalSha(ctx, l.tokenLimitSHA, []string{key},
		tokenLimit,
		periodSeconds,
		tokensToAdd,
	).Result()

	if err != nil {
		return nil, fmt.Errorf("token limit check failed: %w", err)
	}

	// Parse result
	values, ok := result.([]interface{})
	if !ok || len(values) != 4 {
		return nil, fmt.Errorf("unexpected token limit result format")
	}

	status, _ := values[0].(int64)
	used, _ := values[1].(int64)
	remaining, _ := values[2].(int64)
	seconds, _ := values[3].(int64)

	if status == 1 {
		return &TokenLimitResult{
			Allowed:         true,
			TokensUsed:      int(used),
			TokensRemaining: int(remaining),
		}, nil
	}

	return &TokenLimitResult{
		Allowed:         false,
		TokensUsed:      int(used),
		TokensRemaining: int(remaining),
		SecondsToReset:  int(seconds),
	}, nil
}

// GetCurrentUsage returns current token usage without incrementing
func (l *Limiter) GetCurrentUsage(ctx context.Context, guildID, userID string, periodHours int) (int, error) {
	now := time.Now()
	periodSeconds := int64(periodHours * 3600)
	periodStart := (now.Unix() / periodSeconds) * periodSeconds

	key := l.client.Keys().TokenLimit(guildID, userID, periodStart)

	val, err := l.client.Redis().Get(ctx, key).Int()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get usage: %w", err)
	}

	return val, nil
}
