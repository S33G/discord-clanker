package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/s33g/discord-prompter/internal/bot"
	"github.com/s33g/discord-prompter/internal/config"
)

func main() {
	// Parse flags
	configPath := flag.String("config", "config/config.yaml", "Path to configuration file")
	flag.Parse()

	// Setup logger
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	logger := log.With().Str("component", "main").Logger()

	// Load configuration
	logger.Info().Str("path", *configPath).Msg("Loading configuration...")
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Create bot
	logger.Info().Msg("Creating bot...")
	b, err := bot.New(cfg, *configPath, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create bot")
	}

	// Start bot
	logger.Info().Msg("Starting bot...")
	if err := b.Start(); err != nil {
		logger.Fatal().Err(err).Msg("Failed to start bot")
	}

	logger.Info().Msg("Bot is running. Press Ctrl+C to exit.")

	// Wait for interrupt signal
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	// Cleanup
	logger.Info().Msg("Shutting down...")
	b.Stop()
}
