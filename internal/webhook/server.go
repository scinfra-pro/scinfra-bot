package webhook

import (
	"context"
	"log"
	"net/http"
	"time"
)

// TelegramNotifier interface for sending notifications
type TelegramNotifier interface {
	SendNotification(text string) error
}

// Server handles incoming webhooks
type Server struct {
	listenAddr string
	secret     string
	notifier   TelegramNotifier
	httpServer *http.Server
}

// NewServer creates a new webhook server
func NewServer(listenAddr, secret string, notifier TelegramNotifier) *Server {
	return &Server{
		listenAddr: listenAddr,
		secret:     secret,
		notifier:   notifier,
	}
}

// Start starts the webhook server
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook/switch-gate", s.handleSwitchGate)
	mux.HandleFunc("/health", s.handleHealth)

	s.httpServer = &http.Server{
		Addr:         s.listenAddr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("Webhook server starting on %s", s.listenAddr)
	return s.httpServer.ListenAndServe()
}

// Stop gracefully stops the webhook server
func (s *Server) Stop() error {
	if s.httpServer == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Println("Webhook server stopping...")
	return s.httpServer.Shutdown(ctx)
}

// handleHealth returns 200 OK for health checks
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}
