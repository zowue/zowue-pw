package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const (
	apiBase     = "https://portal.qwen.ai/v1"
	authBase    = "https://chat.qwen.ai/api/v1/oauth2"
	clientID    = "f0304373b74a44d2b584a3fb70ca9e56"
	tokenBuffer = 60 * time.Second // refresh 60s before expiry
)

type Client struct {
	httpClient *http.Client
	envPath    string
}

// NewClient creates qwen api client with token auto-refresh from .env
func NewClient() *Client {
	envPath := ".env"
	if path := os.Getenv("ENV_FILE"); path != "" {
		envPath = path
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
		envPath: envPath,
	}
}

// Initialize performs initial authentication check and setup
func (c *Client) Initialize(ctx context.Context) error {
	log.Printf("[AI] initializing qwen client...")

	// ensure authentication exists
	if err := c.ensureAuthenticated(ctx); err != nil {
		return fmt.Errorf("failed to initialize authentication: %w", err)
	}

	// verify token is valid
	if _, err := c.getValidToken(ctx); err != nil {
		return fmt.Errorf("failed to get valid token: %w", err)
	}

	log.Printf("[AI] qwen client initialized successfully")
	return nil
}

// loadTokensFromEnv loads tokens from environment variables
func (c *Client) loadTokensFromEnv() (accessToken, refreshToken string, expiryDate int64, err error) {
	accessToken = os.Getenv("QWEN_ACCESS_TOKEN")
	refreshToken = os.Getenv("QWEN_REFRESH_TOKEN")
	expiryStr := os.Getenv("QWEN_EXPIRY_DATE")

	if accessToken == "" {
		return "", "", 0, fmt.Errorf("QWEN_ACCESS_TOKEN not set in .env")
	}

	if refreshToken == "" {
		return "", "", 0, fmt.Errorf("QWEN_REFRESH_TOKEN not set in .env")
	}

	if expiryStr != "" {
		expiryDate, err = strconv.ParseInt(expiryStr, 10, 64)
		if err != nil {
			return "", "", 0, fmt.Errorf("invalid QWEN_EXPIRY_DATE: %w", err)
		}
	}

	return accessToken, refreshToken, expiryDate, nil
}

// ensureAuthenticated ensures valid tokens exist, triggering device flow if needed
func (c *Client) ensureAuthenticated(ctx context.Context) error {
	// try loading existing tokens
	_, _, _, err := c.loadTokensFromEnv()
	if err == nil {
		return nil
	}

	log.Printf("[AUTH] no valid tokens found: %v", err)
	log.Printf("[AUTH] initiating oauth device flow...")

	// perform device flow authentication
	if err := c.authenticateDeviceFlow(ctx); err != nil {
		return fmt.Errorf("device flow authentication failed: %w", err)
	}

	return nil
}

// updateEnvFile updates tokens in .env file
func (c *Client) updateEnvFile(accessToken, refreshToken string, expiryDate int64) error {
	// read current .env file
	data, err := os.ReadFile(c.envPath)
	if err != nil {
		return fmt.Errorf("failed to read .env file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	updated := make(map[string]bool)

	// update existing lines
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "QWEN_ACCESS_TOKEN=") {
			lines[i] = fmt.Sprintf("QWEN_ACCESS_TOKEN=%s", accessToken)
			updated["access"] = true
		} else if strings.HasPrefix(trimmed, "QWEN_REFRESH_TOKEN=") {
			lines[i] = fmt.Sprintf("QWEN_REFRESH_TOKEN=%s", refreshToken)
			updated["refresh"] = true
		} else if strings.HasPrefix(trimmed, "QWEN_EXPIRY_DATE=") {
			lines[i] = fmt.Sprintf("QWEN_EXPIRY_DATE=%d", expiryDate)
			updated["expiry"] = true
		}
	}

	// add missing variables
	if !updated["access"] {
		lines = append(lines, fmt.Sprintf("QWEN_ACCESS_TOKEN=%s", accessToken))
	}
	if !updated["refresh"] {
		lines = append(lines, fmt.Sprintf("QWEN_REFRESH_TOKEN=%s", refreshToken))
	}
	if !updated["expiry"] {
		lines = append(lines, fmt.Sprintf("QWEN_EXPIRY_DATE=%d", expiryDate))
	}

	// write back to file
	newData := strings.Join(lines, "\n")
	if err := os.WriteFile(c.envPath, []byte(newData), 0600); err != nil {
		return fmt.Errorf("failed to write .env file: %w", err)
	}

	// reload environment
	_ = godotenv.Overload(c.envPath)
	os.Setenv("QWEN_ACCESS_TOKEN", accessToken)
	os.Setenv("QWEN_REFRESH_TOKEN", refreshToken)
	os.Setenv("QWEN_EXPIRY_DATE", strconv.FormatInt(expiryDate, 10))

	return nil
}

// isTokenExpired checks if token needs refresh
func (c *Client) isTokenExpired(expiryDate int64) bool {
	if expiryDate == 0 {
		return true
	}
	// check if token expires in less than 60 seconds
	expiryTime := time.UnixMilli(expiryDate)
	return time.Until(expiryTime) < tokenBuffer
}

// refreshToken refreshes access token using refresh token
func (c *Client) refreshToken(ctx context.Context, refreshToken string) (string, string, int64, error) {
	body := fmt.Sprintf("grant_type=refresh_token&refresh_token=%s&client_id=%s",
		refreshToken, clientID)

	req, err := http.NewRequestWithContext(ctx, "POST", authBase+"/token", bytes.NewBufferString(body))
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to create refresh request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", 0, fmt.Errorf("refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to read refresh response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", "", 0, fmt.Errorf("token refresh failed: %d, body: %s", resp.StatusCode, respBody)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"` // seconds
	}

	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return "", "", 0, fmt.Errorf("failed to parse refresh response: %w", err)
	}

	newExpiryDate := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).UnixMilli()

	// save updated tokens to .env
	if err := c.updateEnvFile(tokenResp.AccessToken, tokenResp.RefreshToken, newExpiryDate); err != nil {
		return "", "", 0, fmt.Errorf("failed to update .env file: %w", err)
	}

	return tokenResp.AccessToken, tokenResp.RefreshToken, newExpiryDate, nil
}

// getValidToken returns valid access token, refreshing if necessary
func (c *Client) getValidToken(ctx context.Context) (string, error) {
	// ensure authentication exists
	if err := c.ensureAuthenticated(ctx); err != nil {
		return "", fmt.Errorf("authentication failed: %w", err)
	}

	// load tokens from env
	accessToken, refreshToken, expiryDate, err := c.loadTokensFromEnv()
	if err != nil {
		log.Printf("[TOKEN] ERROR: failed to load tokens: %v", err)
		return "", err
	}

	log.Printf("[TOKEN] loaded tokens from env, expiry: %d", expiryDate)

	// refresh if expired
	if c.isTokenExpired(expiryDate) {
		log.Printf("[TOKEN] token expired, refreshing...")
		accessToken, _, _, err = c.refreshToken(ctx, refreshToken)
		if err != nil {
			log.Printf("[TOKEN] ERROR: failed to refresh: %v", err)
			return "", fmt.Errorf("failed to refresh token: %w", err)
		}
		log.Printf("[TOKEN] token refreshed successfully")
	} else {
		expiryTime := time.UnixMilli(expiryDate)
		log.Printf("[TOKEN] token valid, expires in %v", time.Until(expiryTime))
	}

	return accessToken, nil
}

// Chat sends messages to qwen api and returns response
func (c *Client) Chat(ctx context.Context, messages []Message, tools []Tool) (*ChatResponse, error) {
	log.Printf("[AI API] getting valid token...")
	token, err := c.getValidToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get valid token: %w", err)
	}
	log.Printf("[AI API] token obtained successfully")

	// prepare request payload
	payload := map[string]interface{}{
		"model":    "coder-model",
		"messages": messages,
		"tools":    tools,
		"stream":   false,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	log.Printf("[AI API] sending request to %s", apiBase+"/chat/completions")
	log.Printf("[AI API] payload size: %d bytes", len(jsonData))
	log.Printf("[AI API] messages count: %d", len(messages))
	log.Printf("[AI API] tools count: %d", len(tools))

	// create http request
	req, err := http.NewRequestWithContext(ctx, "POST", apiBase+"/chat/completions", bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token[:20]+"...")

	// execute request
	startTime := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		log.Printf("[AI API] ERROR: request failed after %v: %v", duration, err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("[AI API] response received in %v, status: %d", duration, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	log.Printf("[AI API] response body size: %d bytes", len(body))

	if resp.StatusCode != http.StatusOK {
		log.Printf("[AI API] ERROR: api error %d, body: %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("api error: %d, body: %s", resp.StatusCode, body)
	}

	// parse response
	var response struct {
		Choices []struct {
			Message struct {
				Role      string     `json:"role"`
				Content   string     `json:"content"`
				ToolCalls []ToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		log.Printf("[AI API] ERROR: failed to parse response: %v", err)
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(response.Choices) == 0 {
		log.Printf("[AI API] ERROR: no choices in response")
		return nil, fmt.Errorf("no response from api")
	}

	log.Printf("[AI API] SUCCESS: parsed response with %d tool calls", len(response.Choices[0].Message.ToolCalls))

	return &ChatResponse{
		Content:   response.Choices[0].Message.Content,
		ToolCalls: response.Choices[0].Message.ToolCalls,
	}, nil
}
