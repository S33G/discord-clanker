package storage

import (
	"context"
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"
	"github.com/s33g/discord-prompter/internal/config"
)

// Client wraps a Redis client with helper methods
type Client struct {
	rdb  *redis.Client
	keys *Keys
}

// NewClient creates a new Redis client from configuration
func NewClient(cfg config.RedisConfig) (*Client, error) {
	// Get password from environment if specified
	password := ""
	if cfg.PasswordEnv != "" {
		password = os.Getenv(cfg.PasswordEnv)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Address,
		Password: password,
		DB:       cfg.DB,
	})

	// Test connection
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Client{
		rdb:  rdb,
		keys: NewKeys(cfg.KeyPrefix),
	}, nil
}

// Close closes the Redis connection
func (c *Client) Close() error {
	return c.rdb.Close()
}

// Ping tests the connection
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Redis returns the underlying Redis client for advanced operations
func (c *Client) Redis() *redis.Client {
	return c.rdb
}

// Keys returns the key generator
func (c *Client) Keys() *Keys {
	return c.keys
}
