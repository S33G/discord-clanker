package config

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Redis: RedisConfig{
			Address:   "localhost:6379",
			DB:        0,
			KeyPrefix: "prompter:",
		},
		Defaults: DefaultsConfig{
			MaxContextTokens:         4096,
			ConversationTTLHours:     168, // 7 days
			UsageRetentionDays:       90,  // 3 months
			MessageHistoryLimit:      50,
			ThreadAutoArchiveMinutes: 60,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}
}
