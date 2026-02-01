package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/meshtastic/meshtastic-go/internal/api"
	"github.com/meshtastic/meshtastic-go/internal/config"
	"github.com/meshtastic/meshtastic-go/internal/database"
	"github.com/meshtastic/meshtastic-go/internal/radio"
	"github.com/meshtastic/meshtastic-go/internal/service"
	"github.com/meshtastic/meshtastic-go/internal/websocket"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	showVersion := flag.Bool("version", false, "show version")
	flag.Parse()

	if *showVersion {
		println("meshtastic-go version", version)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	// Setup logging
	setupLogging(cfg.Logging)

	log.Info().Str("version", version).Msg("starting meshtastic-go")

	// Initialize database
	db, err := database.New(cfg.Database.Path)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize database")
	}
	defer db.Close()

	// Run migrations
	if err := db.Migrate(); err != nil {
		log.Fatal().Err(err).Msg("failed to run migrations")
	}

	// Initialize WebSocket hub
	wsHub := websocket.NewHub()
	go wsHub.Run()

	// Initialize radio manager
	radioManager := radio.NewManager()

	// Initialize mesh service
	meshService := service.NewMeshService(db, radioManager, wsHub)

	// Initialize MQTT bridge
	mqttBridge := service.NewMQTTBridge(cfg.MQTT, wsHub)
	mqttBridge.SetMeshService(meshService)

	// Auto-connect to MQTT if enabled
	if cfg.MQTT.Enabled {
		go func() {
			if err := mqttBridge.Connect(); err != nil {
				log.Error().Err(err).Msg("MQTT bridge connect failed")
			}
		}()
	}

	// Auto-connect to radio if configured
	if cfg.Radio.AutoConnect && cfg.Radio.ConnectionType != "" {
		go func() {
			if err := meshService.Connect(cfg.Radio.ConnectionType, cfg.Radio.Address); err != nil {
				log.Error().Err(err).Msg("auto-connect failed")
			}
		}()
	}

	// Initialize and start HTTP server
	server := api.NewServer(cfg.Server, db, meshService, wsHub, mqttBridge)

	// Handle graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := server.Start(); err != nil {
			log.Fatal().Err(err).Msg("server failed")
		}
	}()

	log.Info().
		Str("host", cfg.Server.Host).
		Int("port", cfg.Server.Port).
		Msg("server started")

	<-ctx.Done()
	log.Info().Msg("shutting down...")

	// Disconnect MQTT bridge
	mqttBridge.Disconnect()

	if err := meshService.Disconnect(); err != nil {
		log.Error().Err(err).Msg("disconnect failed")
	}

	if err := server.Shutdown(context.Background()); err != nil {
		log.Error().Err(err).Msg("server shutdown failed")
	}

	log.Info().Msg("shutdown complete")
}

func setupLogging(cfg config.LoggingConfig) {
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	if cfg.Format == "text" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	if cfg.File != "" {
		f, err := os.OpenFile(cfg.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Error().Err(err).Msg("failed to open log file")
		} else {
			log.Logger = log.Output(f)
		}
	}
}
