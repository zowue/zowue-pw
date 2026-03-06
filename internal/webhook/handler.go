package webhook

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

type Handler struct {
	allowedRepos map[string]bool
	processor    *Processor
}

// NewHandler creates webhook handler with allowed repositories
func NewHandler(repos []string, processor *Processor) *Handler {
	allowed := make(map[string]bool)
	for _, repo := range repos {
		allowed[repo] = true
	}

	return &Handler{
		allowedRepos: allowed,
		processor:    processor,
	}
}

// ServeHTTP handles incoming webhook requests
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// read payload
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("failed to read body: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// parse push event
	var event PushEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("failed to parse payload: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// validate repository
	if !h.allowedRepos[event.Repository.FullName] {
		log.Printf("repository not allowed: %s", event.Repository.FullName)
		w.WriteHeader(http.StatusOK)
		return
	}

	// check if commit message ends with (w)
	if !h.shouldProcess(event.HeadCommit.Message) {
		log.Printf("commit does not end with (w), skipping: %s", event.HeadCommit.Message)
		w.WriteHeader(http.StatusOK)
		return
	}

	log.Printf("processing commit %s from %s: %s",
		event.HeadCommit.ID[:7],
		event.Repository.FullName,
		event.HeadCommit.Message)

	// process asynchronously
	go h.processor.Process(&event)

	w.WriteHeader(http.StatusOK)
}

// shouldProcess checks if commit message ends with (w)
func (h *Handler) shouldProcess(message string) bool {
	trimmed := strings.TrimSpace(message)
	return strings.HasSuffix(trimmed, "(w)")
}

// HealthCheck returns simple health check handler
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}
