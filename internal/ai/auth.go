package ai

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	deviceCodePollInterval = 5 * time.Second
	deviceCodeTimeout      = 15 * time.Minute
)

// pkcePair holds pkce verifier and challenge
type pkcePair struct {
	verifier  string
	challenge string
}

// deviceCodeResponse represents oauth device code response
type deviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// tokenResponse represents oauth token response
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// generatePKCE generates pkce verifier and challenge for oauth flow
func generatePKCE() (*pkcePair, error) {
	// generate 32 random bytes
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// base64url encode verifier
	verifier := base64.RawURLEncoding.EncodeToString(verifierBytes)
	if len(verifier) > 43 {
		verifier = verifier[:43]
	}

	// generate sha256 challenge
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	return &pkcePair{
		verifier:  verifier,
		challenge: challenge,
	}, nil
}

// requestDeviceCode requests device code from oauth server
func (c *Client) requestDeviceCode(ctx context.Context, challenge string) (*deviceCodeResponse, error) {
	body := fmt.Sprintf("client_id=%s&scope=%s&code_challenge=%s&code_challenge_method=S256",
		clientID,
		"openid profile email model.completion",
		challenge)

	req, err := http.NewRequestWithContext(ctx, "POST", authBase+"/device/code", strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create device code request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("device code request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read device code response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device code request failed: %d, body: %s", resp.StatusCode, respBody)
	}

	var deviceResp deviceCodeResponse
	if err := json.Unmarshal(respBody, &deviceResp); err != nil {
		return nil, fmt.Errorf("failed to parse device code response: %w", err)
	}

	return &deviceResp, nil
}

// pollForToken polls oauth server for token after user authorization
func (c *Client) pollForToken(ctx context.Context, deviceCode, verifier string, interval int) (*tokenResponse, error) {
	pollInterval := time.Duration(interval) * time.Second
	if pollInterval < 1*time.Second {
		pollInterval = deviceCodePollInterval
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	timeout := time.After(deviceCodeTimeout)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("device code authorization timeout after %v", deviceCodeTimeout)
		case <-ticker.C:
			token, retry, err := c.attemptTokenPoll(ctx, deviceCode, verifier)
			if err != nil {
				return nil, err
			}
			if !retry {
				return token, nil
			}
			// continue polling
		}
	}
}

// attemptTokenPoll attempts single token poll request
func (c *Client) attemptTokenPoll(ctx context.Context, deviceCode, verifier string) (*tokenResponse, bool, error) {
	body := fmt.Sprintf("grant_type=urn:ietf:params:oauth:grant-type:device_code&client_id=%s&device_code=%s&code_verifier=%s",
		clientID, deviceCode, verifier)

	req, err := http.NewRequestWithContext(ctx, "POST", authBase+"/token", strings.NewReader(body))
	if err != nil {
		return nil, false, fmt.Errorf("failed to create token poll request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("token poll request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read token poll response: %w", err)
	}

	// success case
	if resp.StatusCode == http.StatusOK {
		var tokenResp tokenResponse
		if err := json.Unmarshal(respBody, &tokenResp); err != nil {
			return nil, false, fmt.Errorf("failed to parse token response: %w", err)
		}
		return &tokenResp, false, nil
	}

	// parse error response
	var errorResp struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(respBody, &errorResp); err != nil {
		return nil, false, fmt.Errorf("token poll failed: %d, body: %s", resp.StatusCode, respBody)
	}

	// handle specific error cases
	switch errorResp.Error {
	case "authorization_pending":
		// user hasn't authorized yet, continue polling
		return nil, true, nil
	case "slow_down":
		// server requests slower polling, continue with increased interval
		return nil, true, nil
	default:
		return nil, false, fmt.Errorf("token poll failed: %s - %s", errorResp.Error, errorResp.ErrorDescription)
	}
}

// authenticateDeviceFlow performs full oauth device flow authentication
func (c *Client) authenticateDeviceFlow(ctx context.Context) error {
	// generate pkce pair
	pkce, err := generatePKCE()
	if err != nil {
		return fmt.Errorf("failed to generate pkce: %w", err)
	}

	// request device code
	deviceResp, err := c.requestDeviceCode(ctx, pkce.challenge)
	if err != nil {
		return fmt.Errorf("failed to request device code: %w", err)
	}

	// display authorization url to user
	fmt.Println()
	fmt.Println("QWEN OAUTH AUTHENTICATION REQUIRED")
	fmt.Printf("Open this URL: %s\n", deviceResp.VerificationURIComplete)
	fmt.Printf("User Code: %s\n", deviceResp.UserCode)
	fmt.Println("Waiting for authorization...")
	fmt.Println()

	// poll for token
	tokenResp, err := c.pollForToken(ctx, deviceResp.DeviceCode, pkce.verifier, deviceResp.Interval)
	if err != nil {
		return fmt.Errorf("failed to poll for token: %w", err)
	}

	// calculate expiry date
	expiryDate := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).UnixMilli()

	// save tokens to .env file
	if err := c.updateEnvFile(tokenResp.AccessToken, tokenResp.RefreshToken, expiryDate); err != nil {
		return fmt.Errorf("failed to save tokens: %w", err)
	}

	fmt.Println("Authentication successful, tokens saved to .env")
	fmt.Println()

	return nil
}
