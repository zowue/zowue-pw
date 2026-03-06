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
	tokenBuffer = 60 * time.Second
)

type Client struct {
	httpClient *http.Client
	envPath    string
}

func NewClient() *Client {
	envPath := ".env"
	if path := os.Getenv("ENV_FILE"); path != "" {
		envPath = path
	}

	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Minute},
		envPath:    envPath,
	}
}

func (c *Client) Initialize(ctx context.Context) error {
	if err := c.ensureAuthenticated(ctx); err != nil {
		return err
	}
	if _, err := c.getValidToken(ctx); err != nil {
		return err
	}
	return nil
}

func (c *Client) loadTokensFromEnv() (accessToken, refreshToken string, expiryDate int64, err error) {
	accessToken = os.Getenv("QWEN_ACCESS_TOKEN")
	refreshToken = os.Getenv("QWEN_REFRESH_TOKEN")
	expiryStr := os.Getenv("QWEN_EXPIRY_DATE")

	if accessToken == "" || refreshToken == "" {
		return "", "", 0, fmt.Errorf("tokens not set")
	}

	if expiryStr != "" {
		expiryDate, err = strconv.ParseInt(expiryStr, 10, 64)
		if err != nil {
			return "", "", 0, err
		}
	}

	return accessToken, refreshToken, expiryDate, nil
}

func (c *Client) ensureAuthenticated(ctx context.Context) error {
	if _, _, _, err := c.loadTokensFromEnv(); err == nil {
		return nil
	}
	return c.authenticateDeviceFlow(ctx)
}

func (c *Client) updateEnvFile(accessToken, refreshToken string, expiryDate int64) error {
	data, err := os.ReadFile(c.envPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	updated := make(map[string]bool)

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

	if !updated["access"] {
		lines = append(lines, fmt.Sprintf("QWEN_ACCESS_TOKEN=%s", accessToken))
	}
	if !updated["refresh"] {
		lines = append(lines, fmt.Sprintf("QWEN_REFRESH_TOKEN=%s", refreshToken))
	}
	if !updated["expiry"] {
		lines = append(lines, fmt.Sprintf("QWEN_EXPIRY_DATE=%d", expiryDate))
	}

	newData := strings.Join(lines, "\n")
	if err := os.WriteFile(c.envPath, []byte(newData), 0600); err != nil {
		return err
	}

	_ = godotenv.Overload(c.envPath)
	os.Setenv("QWEN_ACCESS_TOKEN", accessToken)
	os.Setenv("QWEN_REFRESH_TOKEN", refreshToken)
	os.Setenv("QWEN_EXPIRY_DATE", strconv.FormatInt(expiryDate, 10))

	return nil
}

func (c *Client) isTokenExpired(expiryDate int64) bool {
	if expiryDate == 0 {
		return true
	}
	return time.Until(time.UnixMilli(expiryDate)) < tokenBuffer
}

func (c *Client) refreshToken(ctx context.Context, refreshToken string) (string, string, int64, error) {
	body := fmt.Sprintf("grant_type=refresh_token&refresh_token=%s&client_id=%s", refreshToken, clientID)

	req, err := http.NewRequestWithContext(ctx, "POST", authBase+"/token", strings.NewReader(body))
	if err != nil {
		return "", "", 0, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", 0, err
	}

	if resp.StatusCode != http.StatusOK {
		return "", "", 0, fmt.Errorf("refresh failed: %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}

	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return "", "", 0, err
	}

	newExpiryDate := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).UnixMilli()

	if err := c.updateEnvFile(tokenResp.AccessToken, tokenResp.RefreshToken, newExpiryDate); err != nil {
		return "", "", 0, err
	}

	return tokenResp.AccessToken, tokenResp.RefreshToken, newExpiryDate, nil
}

func (c *Client) getValidToken(ctx context.Context) (string, error) {
	if err := c.ensureAuthenticated(ctx); err != nil {
		return "", err
	}

	accessToken, refreshToken, expiryDate, err := c.loadTokensFromEnv()
	if err != nil {
		return "", err
	}

	if c.isTokenExpired(expiryDate) {
		accessToken, _, _, err = c.refreshToken(ctx, refreshToken)
		if err != nil {
			return "", err
		}
	}

	return accessToken, nil
}

func (c *Client) Chat(ctx context.Context, messages []Message, tools []Tool) (*ChatResponse, error) {
	const maxRetries = 2

	for attempt := 0; attempt < maxRetries; attempt++ {
		token, err := c.getValidToken(ctx)
		if err != nil {
			return nil, err
		}

		payload := map[string]interface{}{
			"model":    "coder-model",
			"messages": messages,
			"tools":    tools,
			"stream":   false,
		}

		jsonData, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, "POST", apiBase+"/chat/completions", bytes.NewReader(jsonData))
		if err != nil {
			return nil, err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		// check for quota exceeded or invalid token error
		if resp.StatusCode == 429 || resp.StatusCode == 401 {
			if attempt < maxRetries-1 {
				log.Printf("quota exceeded or token invalid, refreshing token and retrying...")
				// force token refresh
				_, refreshToken, _, err := c.loadTokensFromEnv()
				if err != nil {
					return nil, fmt.Errorf("failed to load refresh token: %w", err)
				}
				_, _, _, err = c.refreshToken(ctx, refreshToken)
				if err != nil {
					return nil, fmt.Errorf("failed to refresh token: %w", err)
				}
				continue
			}
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("api error %d: %s", resp.StatusCode, body)
		}

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
			return nil, err
		}

		if len(response.Choices) == 0 {
			return nil, fmt.Errorf("no response")
		}

		return &ChatResponse{
			Content:   response.Choices[0].Message.Content,
			ToolCalls: response.Choices[0].Message.ToolCalls,
		}, nil
	}

	return nil, fmt.Errorf("max retries exceeded")
}
