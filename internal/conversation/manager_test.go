package conversation

import (
	"context"
	"testing"
	"time"

	"github.com/s33g/discord-prompter/internal/config"
	"github.com/s33g/discord-prompter/internal/storage"
)

// Note: These tests require a running Redis instance
// Set REDIS_TEST_ADDR to point to test Redis (default: localhost:6379)
// Run: docker run -d -p 6379:6379 redis:7-alpine

func getTestClient(t *testing.T) *storage.Client {
	t.Helper()

	cfg := config.RedisConfig{
		Address:   "localhost:6379",
		DB:        15, // Use DB 15 for testing
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

func TestManager_CreateAndGet(t *testing.T) {
	client := getTestClient(t)
	defer client.Close()

	mgr := NewManager(client, time.Hour, 50)
	ctx := context.Background()

	conv := Conversation{
		ThreadID:     "thread123",
		GuildID:      "guild456",
		ChannelID:    "channel789",
		UserID:       "user000",
		Model:        "test/model",
		SystemPrompt: "You are a test assistant.",
		Title:        "Test Conversation",
		TokenCount:   0,
	}

	// Create conversation
	if err := mgr.Create(ctx, conv); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Get conversation
	got, err := mgr.Get(ctx, "guild456", "thread123")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if got.ThreadID != conv.ThreadID {
		t.Errorf("ThreadID = %v, want %v", got.ThreadID, conv.ThreadID)
	}
	if got.Model != conv.Model {
		t.Errorf("Model = %v, want %v", got.Model, conv.Model)
	}
	if got.Title != conv.Title {
		t.Errorf("Title = %v, want %v", got.Title, conv.Title)
	}
}

func TestManager_AddAndGetMessages(t *testing.T) {
	client := getTestClient(t)
	defer client.Close()

	mgr := NewManager(client, time.Hour, 50)
	ctx := context.Background()

	// Create conversation
	conv := Conversation{
		ThreadID: "thread123",
		GuildID:  "guild456",
		Model:    "test/model",
	}
	if err := mgr.Create(ctx, conv); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Add messages
	messages := []Message{
		{Role: "system", Content: "You are helpful.", Tokens: 5},
		{Role: "user", Content: "Hello!", Tokens: 3, MessageID: "msg1"},
		{Role: "assistant", Content: "Hi there!", Tokens: 4, MessageID: "msg2"},
	}

	for _, msg := range messages {
		if err := mgr.AddMessage(ctx, "guild456", "thread123", msg); err != nil {
			t.Fatalf("AddMessage() error = %v", err)
		}
	}

	// Get messages
	got, err := mgr.GetMessages(ctx, "guild456", "thread123")
	if err != nil {
		t.Fatalf("GetMessages() error = %v", err)
	}

	if len(got) != len(messages) {
		t.Fatalf("Got %d messages, want %d", len(got), len(messages))
	}

	for i, want := range messages {
		if got[i].Role != want.Role {
			t.Errorf("Message %d: Role = %v, want %v", i, got[i].Role, want.Role)
		}
		if got[i].Content != want.Content {
			t.Errorf("Message %d: Content = %v, want %v", i, got[i].Content, want.Content)
		}
	}
}

func TestManager_ClearMessages(t *testing.T) {
	client := getTestClient(t)
	defer client.Close()

	mgr := NewManager(client, time.Hour, 50)
	ctx := context.Background()

	// Create conversation with messages
	conv := Conversation{
		ThreadID: "thread123",
		GuildID:  "guild456",
		Model:    "test/model",
	}
	mgr.Create(ctx, conv)
	mgr.AddMessage(ctx, "guild456", "thread123", Message{Role: "user", Content: "Test"})

	// Clear messages
	if err := mgr.ClearMessages(ctx, "guild456", "thread123"); err != nil {
		t.Fatalf("ClearMessages() error = %v", err)
	}

	// Verify messages are cleared
	messages, err := mgr.GetMessages(ctx, "guild456", "thread123")
	if err != nil {
		t.Fatalf("GetMessages() error = %v", err)
	}

	if len(messages) != 0 {
		t.Errorf("Got %d messages after clear, want 0", len(messages))
	}

	// Verify conversation still exists
	_, err = mgr.Get(ctx, "guild456", "thread123")
	if err != nil {
		t.Errorf("Conversation should still exist after clearing messages")
	}
}

func TestManager_UpdateModel(t *testing.T) {
	client := getTestClient(t)
	defer client.Close()

	mgr := NewManager(client, time.Hour, 50)
	ctx := context.Background()

	// Create conversation
	conv := Conversation{
		ThreadID: "thread123",
		GuildID:  "guild456",
		Model:    "test/model1",
	}
	mgr.Create(ctx, conv)

	// Update model
	if err := mgr.UpdateModel(ctx, "guild456", "thread123", "test/model2"); err != nil {
		t.Fatalf("UpdateModel() error = %v", err)
	}

	// Verify update
	got, _ := mgr.Get(ctx, "guild456", "thread123")
	if got.Model != "test/model2" {
		t.Errorf("Model = %v, want test/model2", got.Model)
	}
}

func TestManager_MessageTrimming(t *testing.T) {
	client := getTestClient(t)
	defer client.Close()

	// Create manager with small max messages
	mgr := NewManager(client, time.Hour, 3)
	ctx := context.Background()

	// Create conversation
	conv := Conversation{
		ThreadID: "thread123",
		GuildID:  "guild456",
		Model:    "test/model",
	}
	mgr.Create(ctx, conv)

	// Add more messages than max
	for i := 0; i < 5; i++ {
		msg := Message{
			Role:    "user",
			Content: string(rune('A' + i)), // A, B, C, D, E
		}
		mgr.AddMessage(ctx, "guild456", "thread123", msg)
	}

	// Should only keep last 3 messages
	messages, _ := mgr.GetMessages(ctx, "guild456", "thread123")
	if len(messages) != 3 {
		t.Fatalf("Got %d messages, want 3", len(messages))
	}

	// Should be C, D, E (last 3)
	expected := []string{"C", "D", "E"}
	for i, want := range expected {
		if messages[i].Content != want {
			t.Errorf("Message %d: Content = %v, want %v", i, messages[i].Content, want)
		}
	}
}
