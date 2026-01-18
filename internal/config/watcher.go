package config

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog"
)

// Watcher watches for configuration file changes
type Watcher struct {
	configPath string
	logger     zerolog.Logger
	watcher    *fsnotify.Watcher
	reloadFunc func(*Config) error
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewWatcher creates a new config file watcher
func NewWatcher(configPath string, reloadFunc func(*Config) error, logger zerolog.Logger) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	// Add the config file to the watcher
	err = fsWatcher.Add(configPath)
	if err != nil {
		fsWatcher.Close()
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	w := &Watcher{
		configPath: configPath,
		logger:     logger,
		watcher:    fsWatcher,
		reloadFunc: reloadFunc,
		ctx:        ctx,
		cancel:     cancel,
	}

	return w, nil
}

// Start starts watching for config changes
func (w *Watcher) Start() {
	// Setup signal handler for SIGHUP
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP)

	go func() {
		defer w.watcher.Close()

		// Debounce timer to avoid multiple rapid reloads
		var debounceTimer *time.Timer
		const debounceDelay = 500 * time.Millisecond

		for {
			select {
			case <-w.ctx.Done():
				w.logger.Info().Msg("Config watcher stopped")
				return

			case sig := <-sigChan:
				w.logger.Info().
					Str("signal", sig.String()).
					Msg("Received signal, reloading configuration")
				w.reload()

			case event, ok := <-w.watcher.Events:
				if !ok {
					return
				}

				// Only handle Write and Create events
				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
					w.logger.Debug().
						Str("file", event.Name).
						Str("op", event.Op.String()).
						Msg("Config file changed")

					// Debounce the reload - cancel existing timer if any
					if debounceTimer != nil {
						debounceTimer.Stop()
					}

					debounceTimer = time.AfterFunc(debounceDelay, func() {
						w.reload()
					})
				}

			case err, ok := <-w.watcher.Errors:
				if !ok {
					return
				}
				w.logger.Error().Err(err).Msg("Config watcher error")
			}
		}
	}()

	w.logger.Info().
		Str("path", w.configPath).
		Msg("Config watcher started")
}

// Stop stops the watcher
func (w *Watcher) Stop() {
	w.cancel()
}

// reload loads and applies the new configuration
func (w *Watcher) reload() {
	w.logger.Info().Msg("Reloading configuration...")

	// Load new config
	newCfg, err := Load(w.configPath)
	if err != nil {
		w.logger.Error().
			Err(err).
			Msg("Failed to load new configuration - keeping current config")
		return
	}

	// Validate the new config
	if err := newCfg.Validate(); err != nil {
		w.logger.Error().
			Err(err).
			Msg("New configuration is invalid - keeping current config")
		return
	}

	// Apply the new config
	if err := w.reloadFunc(newCfg); err != nil {
		w.logger.Error().
			Err(err).
			Msg("Failed to apply new configuration - keeping current config")
		return
	}

	w.logger.Info().Msg("Configuration reloaded successfully")
}
