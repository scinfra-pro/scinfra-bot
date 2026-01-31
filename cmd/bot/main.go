package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/scinfra-pro/scinfra-bot/internal/config"
	"github.com/scinfra-pro/scinfra-bot/internal/edge"
	"github.com/scinfra-pro/scinfra-bot/internal/telegram"
	"github.com/scinfra-pro/scinfra-bot/internal/webhook"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "/etc/scinfra-bot/config.yaml", "config file path")
	showVersion := flag.Bool("version", false, "show version")
	flag.Parse()

	if *showVersion {
		log.Printf("scinfra-bot %s", version)
		os.Exit(0)
	}

	log.Printf("scinfra-bot %s starting...", version)

	// Load configuration from YAML
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Load metadata from S3 (if enabled)
	if cfg.S3.Enabled {
		log.Printf("Loading infrastructure metadata from S3...")
		s3Loader, err := config.NewS3Loader(cfg.S3)
		if err != nil {
			log.Printf("Warning: S3 loader init failed: %v (using YAML config)", err)
		} else if s3Loader != nil {
			metadata, err := s3Loader.Load(context.Background(), cfg.S3.Providers)
			if err != nil {
				log.Printf("Warning: S3 metadata load failed: %v (using YAML config)", err)
			} else {
				cfg.MergeS3Metadata(metadata)
				log.Printf("S3 metadata loaded: %d upstreams, %d clouds",
					len(cfg.Upstreams), len(cfg.Infrastructure.Clouds))
			}
		}
	}

	// Validate runtime config (after S3 merge)
	if err := cfg.ValidateRuntime(); err != nil {
		log.Fatalf("Config validation failed: %v", err)
	}

	// Initialize edge client
	edgeClient, err := edge.New(
		cfg.Edge.Host,
		cfg.Edge.KeyPath,
		cfg.Edge.VPNModeScript,
	)
	if err != nil {
		log.Fatalf("Failed to create edge client: %v", err)
	}

	// Initialize Telegram bot
	bot, err := telegram.New(cfg, edgeClient)
	if err != nil {
		log.Fatalf("Failed to create Telegram bot: %v", err)
	}

	// Initialize webhook server (if enabled)
	var webhookServer *webhook.Server
	if cfg.Webhooks.Enabled {
		webhookServer = webhook.NewServer(
			cfg.Webhooks.Listen,
			cfg.Webhooks.Secret,
			bot,
		)
		log.Printf("Webhook receiver enabled on %s", cfg.Webhooks.Listen)
	}

	// Graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Start bot in goroutine
	go func() {
		if err := bot.Start(); err != nil {
			log.Fatalf("Bot error: %v", err)
		}
	}()

	// Start webhook server in goroutine (if enabled)
	if webhookServer != nil {
		go func() {
			if err := webhookServer.Start(); err != nil {
				log.Printf("Webhook server error: %v", err)
			}
		}()
	}

	// Wait for shutdown signal
	<-ctx.Done()
	log.Println("Shutting down...")

	// Stop webhook server
	if webhookServer != nil {
		if err := webhookServer.Stop(); err != nil {
			log.Printf("Error stopping webhook server: %v", err)
		}
	}

	bot.Stop()
	log.Println("Goodbye!")
}
