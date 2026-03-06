package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/zarazaex69/zowue-analyzer/internal/ai"
	"github.com/zarazaex69/zowue-analyzer/internal/config"
	"github.com/zarazaex69/zowue-analyzer/internal/server"
)

func main() {
	// load .env file if exists
	_ = godotenv.Load()

	// load configuration from environment
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// initialize ai agent with authentication
	log.Println("initializing ai agent...")
	agent := ai.NewAgent()
	ctx := context.Background()
	if err := agent.Initialize(ctx); err != nil {
		log.Fatalf("failed to initialize ai agent: %v", err)
	}

	// create webhook server
	srv := server.New(cfg)

	// setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("shutdown signal received")
		cancel()
	}()

	// start server
	log.Printf("starting webhook server on port %s", cfg.WebhookPort)
	if err := srv.Start(ctx); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
