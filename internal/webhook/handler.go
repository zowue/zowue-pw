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
	log.Printf("received webhook request: method=%s, path=%s", r.Method, r.URL.Path)

	if r.Method != http.MethodPost {
		log.Printf("rejected: method not allowed: %s", r.Method)
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

	log.Printf("received payload: %d bytes", len(body))

	// parse push event
	var event PushEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("failed to parse payload: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// validate commit data
	if event.HeadCommit.ID == "" {
		log.Printf("rejected: empty commit id")
		w.WriteHeader(http.StatusOK)
		return
	}

	commitShort := event.HeadCommit.ID
	if len(commitShort) > 7 {
		commitShort = commitShort[:7]
	}

	log.Printf("parsed event: repo=%s, commit=%s, message=%s",
		event.Repository.FullName,
		commitShort,
		event.HeadCommit.Message)

	// validate repository
	if !h.allowedRepos[event.Repository.FullName] {
		log.Printf("repository not allowed: %s (allowed: %v)", event.Repository.FullName, h.getAllowedReposList())
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
		commitShort,
		event.Repository.FullName,
		event.HeadCommit.Message)

	// process asynchronously
	go h.processor.Process(&event)

	w.WriteHeader(http.StatusOK)
}

// getAllowedReposList returns list of allowed repos for logging
func (h *Handler) getAllowedReposList() []string {
	repos := make([]string, 0, len(h.allowedRepos))
	for repo := range h.allowedRepos {
		repos = append(repos, repo)
	}
	return repos
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
