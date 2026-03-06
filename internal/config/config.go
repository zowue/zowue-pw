package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	GitHubToken  string
	Repositories []string
	WebhookPort  string
	WorkDir      string
}

// load reads configuration from environment variables
func Load() (*Config, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN is required")
	}

	repoStr := os.Getenv("REPO")
	if repoStr == "" {
		return nil, fmt.Errorf("REPO is required")
	}

	repos := strings.Split(repoStr, ",")
	for i, repo := range repos {
		repos[i] = strings.TrimSpace(repo)
	}

	port := os.Getenv("GITHUB_WEBHOOK_PORT")
	if port == "" {
		port = "8802"
	}

	workDir := os.Getenv("WORK_DIR")
	if workDir == "" {
		workDir = "/tmp/zowue-pw"
	}

	return &Config{
		GitHubToken:  token,
		Repositories: repos,
		WebhookPort:  port,
		WorkDir:      workDir,
	}, nil
}
