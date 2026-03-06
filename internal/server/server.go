package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/zarazaex69/zowue-pw/internal/config"
	"github.com/zarazaex69/zowue-pw/internal/webhook"
)

type Server struct {
	cfg    *config.Config
	server *http.Server
}

// New creates webhook server
func New(cfg *config.Config) *Server {
	processor := webhook.NewProcessor(cfg.WorkDir, cfg.GitHubToken)
	handler := webhook.NewHandler(cfg.Repositories, processor)

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", handler.ServeHTTP)
	mux.HandleFunc("/health", webhook.HealthCheck)

	return &Server{
		cfg: cfg,
		server: &http.Server{
			Addr:         ":" + cfg.WebhookPort,
			Handler:      mux,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
	}
}

// Start runs webhook server
func (s *Server) Start(ctx context.Context) error {
	errChan := make(chan error, 1)

	go func() {
		log.Printf("webhook server listening on %s", s.server.Addr)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case err := <-errChan:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		log.Println("shutting down server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.server.Shutdown(shutdownCtx)
	}
}
