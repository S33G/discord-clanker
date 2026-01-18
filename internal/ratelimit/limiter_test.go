package ratelimit

import (
	"context"
	"testing"

	"github.com/s33g/discord-prompter/internal/config"
	"github.com/s33g/discord-prompter/internal/storage"
)

func getTestClient(t *testing.T) *storage.Client {
	t.Helper()

	cfg := config.RedisConfig{
		Address:   "localhost:6379",
		DB:        15,
		KeyPrefix: "test:",
	}

	client, err := storage.NewClient(cfg)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	// Clean test database
	ctx := context.Background()
	client.Redis().FlushDB(ctx)

	return client
}

func TestLimiter_CheckRateLimit(t *testing.T) {
	client := getTestClient(t)
	defer client.Close()

	limiter, err := NewLimiter(client)
	if err != nil {
		t.Fatalf("NewLimiter() error = %v", err)
	}

	ctx := context.Background()
	limits := config.RateLimit{
		RequestsPerMinute: 3,
		RequestsPerHour:   10,
	}

	// First 3 requests should succeed
	for i := 0; i < 3; i++ {
		result, err := limiter.CheckRateLimit(ctx, "guild1", "user1", limits)
		if err != nil {
			t.Fatalf("CheckRateLimit() error = %v", err)
		}
		if !result.Allowed {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// 4th request should be rate limited
	result, err := limiter.CheckRateLimit(ctx, "guild1", "user1", limits)
	if err != nil {
		t.Fatalf("CheckRateLimit() error = %v", err)
	}
	if result.Allowed {
		t.Error("4th request should be rate limited")
	}
	if result.LimitType != "minute" {
		t.Errorf("LimitType = %v, want minute", result.LimitType)
	}
	if result.SecondsToReset <= 0 {
		t.Error("SecondsToReset should be positive")
	}
}

func TestLimiter_CheckRateLimitUnlimited(t *testing.T) {
	client := getTestClient(t)
	defer client.Close()

	limiter, err := NewLimiter(client)
	if err != nil {
		t.Fatalf("NewLimiter() error = %v", err)
	}

	ctx := context.Background()
	limits := config.RateLimit{
		RequestsPerMinute: 0, // Unlimited
		RequestsPerHour:   0,
	}

	// Many requests should all succeed
	for i := 0; i < 100; i++ {
		result, err := limiter.CheckRateLimit(ctx, "guild1", "user1", limits)
		if err != nil {
			t.Fatalf("CheckRateLimit() error = %v", err)
		}
		if !result.Allowed {
			t.Errorf("Request %d should be allowed (unlimited)", i+1)
		}
	}
}

func TestLimiter_CheckTokenLimit(t *testing.T) {
	client := getTestClient(t)
	defer client.Close()

	limiter, err := NewLimiter(client)
	if err != nil {
		t.Fatalf("NewLimiter() error = %v", err)
	}

	ctx := context.Background()
	limit := config.TokenLimit{
		TokensPerPeriod: 100,
		PeriodHours:     1,
	}

	// Add tokens incrementally
	result, err := limiter.CheckTokenLimit(ctx, "guild1", "user1", limit, 30)
	if err != nil {
		t.Fatalf("CheckTokenLimit() error = %v", err)
	}
	if !result.Allowed {
		t.Error("First token add should be allowed")
	}
	if result.TokensUsed != 30 {
		t.Errorf("TokensUsed = %d, want 30", result.TokensUsed)
	}

	// Add more tokens
	result, err = limiter.CheckTokenLimit(ctx, "guild1", "user1", limit, 50)
	if err != nil {
		t.Fatalf("CheckTokenLimit() error = %v", err)
	}
	if !result.Allowed {
		t.Error("Second token add should be allowed")
	}
	if result.TokensUsed != 80 {
		t.Errorf("TokensUsed = %d, want 80", result.TokensUsed)
	}

	// Try to exceed limit
	result, err = limiter.CheckTokenLimit(ctx, "guild1", "user1", limit, 30)
	if err != nil {
		t.Fatalf("CheckTokenLimit() error = %v", err)
	}
	if result.Allowed {
		t.Error("Token add should be rejected (would exceed limit)")
	}
	if result.TokensUsed != 80 {
		t.Errorf("TokensUsed = %d, want 80 (unchanged)", result.TokensUsed)
	}
	if result.SecondsToReset <= 0 {
		t.Error("SecondsToReset should be positive")
	}
}

func TestLimiter_CheckTokenLimitBypass(t *testing.T) {
	client := getTestClient(t)
	defer client.Close()

	limiter, err := NewLimiter(client)
	if err != nil {
		t.Fatalf("NewLimiter() error = %v", err)
	}

	ctx := context.Background()
	limit := config.TokenLimit{
		Bypass: true, // Unlimited
	}

	// Large token amounts should all succeed
	for i := 0; i < 10; i++ {
		result, err := limiter.CheckTokenLimit(ctx, "guild1", "user1", limit, 10000)
		if err != nil {
			t.Fatalf("CheckTokenLimit() error = %v", err)
		}
		if !result.Allowed {
			t.Errorf("Token add %d should be allowed (bypass)", i+1)
		}
	}
}

func TestLimiter_GetCurrentUsage(t *testing.T) {
	client := getTestClient(t)
	defer client.Close()

	limiter, err := NewLimiter(client)
	if err != nil {
		t.Fatalf("NewLimiter() error = %v", err)
	}

	ctx := context.Background()

	// Initially should be 0
	usage, err := limiter.GetCurrentUsage(ctx, "guild1", "user1", 1)
	if err != nil {
		t.Fatalf("GetCurrentUsage() error = %v", err)
	}
	if usage != 0 {
		t.Errorf("Initial usage = %d, want 0", usage)
	}

	// Add some tokens
	limit := config.TokenLimit{
		TokensPerPeriod: 1000,
		PeriodHours:     1,
	}
	limiter.CheckTokenLimit(ctx, "guild1", "user1", limit, 50)

	// Check usage
	usage, err = limiter.GetCurrentUsage(ctx, "guild1", "user1", 1)
	if err != nil {
		t.Fatalf("GetCurrentUsage() error = %v", err)
	}
	if usage != 50 {
		t.Errorf("Usage = %d, want 50", usage)
	}
}

func TestLimiter_DifferentUsers(t *testing.T) {
	client := getTestClient(t)
	defer client.Close()

	limiter, err := NewLimiter(client)
	if err != nil {
		t.Fatalf("NewLimiter() error = %v", err)
	}

	ctx := context.Background()
	limits := config.RateLimit{
		RequestsPerMinute: 2,
		RequestsPerHour:   10,
	}

	// User1 uses their quota
	limiter.CheckRateLimit(ctx, "guild1", "user1", limits)
	limiter.CheckRateLimit(ctx, "guild1", "user1", limits)

	result, _ := limiter.CheckRateLimit(ctx, "guild1", "user1", limits)
	if result.Allowed {
		t.Error("user1 should be rate limited")
	}

	// User2 should still have quota
	result, _ = limiter.CheckRateLimit(ctx, "guild1", "user2", limits)
	if !result.Allowed {
		t.Error("user2 should not be rate limited")
	}
}
