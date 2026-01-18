package bot

import (
	"context"
	"fmt"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog"
	"github.com/s33g/discord-prompter/internal/config"
	"github.com/s33g/discord-prompter/internal/conversation"
	"github.com/s33g/discord-prompter/internal/llm"
	"github.com/s33g/discord-prompter/internal/ratelimit"
	"github.com/s33g/discord-prompter/internal/rbac"
	"github.com/s33g/discord-prompter/internal/storage"
)

// Bot represents the Discord bot
type Bot struct {
	session       *discordgo.Session
	config        *config.Config
	configPath    string // Path to config file for reload
	configWatcher *config.Watcher
	configMu      sync.RWMutex
	storage       *storage.Client
	llmRegistry   *llm.Registry
	rbacManager   *rbac.Manager
	rateLimiter   *ratelimit.Limiter
	convManager   *conversation.Manager
	logger        zerolog.Logger
	ctx           context.Context
	cancel        context.CancelFunc
}

// New creates a new bot instance
func New(cfg *config.Config, configPath string, logger zerolog.Logger) (*Bot, error) {
	// Create Discord session
	session, err := discordgo.New("Bot " + cfg.Discord.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}

	// Set intents
	session.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildMessages |
		discordgo.IntentsMessageContent |
		discordgo.IntentsGuildMembers

	// Connect to Redis
	storageClient, err := storage.NewClient(cfg.Redis)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Initialize LLM registry
	llmRegistry, err := llm.NewRegistry(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize LLM registry: %w", err)
	}

	// Initialize RBAC manager
	rbacManager := rbac.NewManager(cfg)

	// Initialize rate limiter
	rateLimiter, err := ratelimit.NewLimiter(storageClient)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize rate limiter: %w", err)
	}

	// Initialize conversation manager
	// Use default guild's settings for now (multi-guild support will vary per operation)
	defaultTTL := cfg.Defaults.ConversationTTL()
	convManager := conversation.NewManager(storageClient, defaultTTL, cfg.Defaults.MessageHistoryLimit)

	ctx, cancel := context.WithCancel(context.Background())

	bot := &Bot{
		session:     session,
		config:      cfg,
		configPath:  configPath,
		storage:     storageClient,
		llmRegistry: llmRegistry,
		rbacManager: rbacManager,
		rateLimiter: rateLimiter,
		convManager: convManager,
		logger:      logger,
		ctx:         ctx,
		cancel:      cancel,
	}

	// Register handlers
	bot.registerHandlers()

	// Setup config watcher
	watcher, err := config.NewWatcher(configPath, bot.Reload, logger)
	if err != nil {
		// Non-fatal - just log the error
		bot.logger.Warn().Err(err).Msg("Failed to create config watcher - hot reload disabled")
	} else {
		bot.configWatcher = watcher
	}

	return bot, nil
}

// Start starts the bot
func (b *Bot) Start() error {
	b.logger.Info().Msg("Starting Discord bot...")

	// Open connection
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("failed to open Discord session: %w", err)
	}

	// Register commands
	if err := b.registerCommands(); err != nil {
		return fmt.Errorf("failed to register commands: %w", err)
	}

	// Start config watcher
	if b.configWatcher != nil {
		b.configWatcher.Start()
	}

	b.logger.Info().Msg("Bot started successfully")
	return nil
}

// Stop stops the bot
func (b *Bot) Stop() error {
	b.logger.Info().Msg("Stopping Discord bot...")

	// Stop config watcher
	if b.configWatcher != nil {
		b.configWatcher.Stop()
	}

	b.cancel()

	// Close Discord session
	if err := b.session.Close(); err != nil {
		b.logger.Error().Err(err).Msg("Failed to close Discord session")
	}

	// Close Redis connection
	if err := b.storage.Close(); err != nil {
		b.logger.Error().Err(err).Msg("Failed to close Redis connection")
	}

	b.logger.Info().Msg("Bot stopped")
	return nil
}

// Reload reloads the bot configuration
func (b *Bot) Reload(cfg *config.Config) error {
	b.configMu.Lock()
	defer b.configMu.Unlock()

	// Reload LLM registry
	if err := b.llmRegistry.Reload(cfg); err != nil {
		return fmt.Errorf("failed to reload LLM registry: %w", err)
	}

	// Reload RBAC manager
	if err := b.rbacManager.Reload(cfg); err != nil {
		return fmt.Errorf("failed to reload RBAC manager: %w", err)
	}

	// Update config
	b.config = cfg

	b.logger.Info().Msg("Configuration reloaded successfully")
	return nil
}

// GetConfig safely returns the current configuration
func (b *Bot) GetConfig() *config.Config {
	b.configMu.RLock()
	defer b.configMu.RUnlock()
	return b.config
}

// registerHandlers registers Discord event handlers
func (b *Bot) registerHandlers() {
	b.session.AddHandler(b.handleInteractionCreate)
	b.session.AddHandler(b.handleMessageCreate)
	b.session.AddHandler(b.handleReady)
}

// handleReady is called when the bot is ready
func (b *Bot) handleReady(s *discordgo.Session, r *discordgo.Ready) {
	b.logger.Info().
		Str("username", r.User.Username).
		Int("guilds", len(r.Guilds)).
		Msg("Bot is ready")
}

// registerCommands registers slash commands with Discord
func (b *Bot) registerCommands() error {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "ask",
			Description: "Start a new conversation with an LLM",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "prompt",
					Description: "Your question or prompt",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "model",
					Description: "Model to use (optional, uses default if not specified)",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "system_prompt",
					Description: "System prompt to use (optional, uses default if not specified)",
					Required:    false,
				},
			},
		},
		{
			Name:        "models",
			Description: "List available models",
		},
		{
			Name:        "prompts",
			Description: "List available system prompts",
		},
		{
			Name:        "usage",
			Description: "Show your usage statistics",
		},
		{
			Name:        "reload",
			Description: "Reload bot configuration (requires permission)",
		},
	}

	cfg := b.GetConfig()

	// Register commands for each guild
	for _, guildCfg := range cfg.Guilds {
		for _, cmd := range commands {
			_, err := b.session.ApplicationCommandCreate(b.session.State.User.ID, guildCfg.ID, cmd)
			if err != nil {
				b.logger.Error().
					Err(err).
					Str("guild", guildCfg.ID).
					Str("command", cmd.Name).
					Msg("Failed to register command")
				return fmt.Errorf("failed to register command %s for guild %s: %w", cmd.Name, guildCfg.ID, err)
			}
		}
		b.logger.Info().
			Str("guild", guildCfg.Name).
			Int("commands", len(commands)).
			Msg("Registered slash commands")
	}

	return nil
}
