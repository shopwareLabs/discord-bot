package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"discord-sso-role/models"
	"discord-sso-role/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type OAuthHandler struct {
	config *models.Config
	store  *models.VerificationStore
}

func NewOAuthHandler(config *models.Config, store *models.VerificationStore) *OAuthHandler {
	return &OAuthHandler{
		config: config,
		store:  store,
	}
}

// StartAuth initiates the OAuth flow
func (h *OAuthHandler) StartAuth(c *gin.Context) {
	state := c.Query("state") // Discord ID passed as state
	if state == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing state parameter"})
		return
	}

	authURL := fmt.Sprintf(
		"https://login.microsoftonline.com/%s/oauth2/v2.0/authorize?"+
			"client_id=%s&"+
			"response_type=code&"+
			"redirect_uri=%s&"+
			"response_mode=query&"+
			"scope=openid%%20email%%20profile&"+
			"state=%s",
		h.config.MicrosoftTenantID,
		h.config.MicrosoftClientID,
		h.config.MicrosoftRedirectURL,
		state,
	)

	c.Redirect(http.StatusTemporaryRedirect, authURL)
}

// Callback handles the OAuth callback
func (h *OAuthHandler) Callback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state") // Discord ID
	
	if code == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": "Authorization code not provided",
		})
		return
	}

	// Exchange code for token
	token, err := h.exchangeCodeForToken(code)
	if err != nil {
		utils.Logger.Printf("Failed to exchange code for token: %v", err)
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to authenticate with Microsoft",
		})
		return
	}

	// Get user info
	userInfo, err := h.getUserInfo(token.AccessToken)
	if err != nil {
		utils.Logger.Printf("Failed to get user info: %v", err)
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to get user information",
		})
		return
	}

	// Generate verification code
	verificationCode := &models.VerificationCode{
		Code:      uuid.New().String()[:8],
		Email:     userInfo.Email,
		DiscordID: state,
		ExpiresAt: time.Now().Add(time.Duration(h.config.VerificationTTL) * time.Minute),
	}

	// Store verification code
	if err := h.store.Store(verificationCode); err != nil {
		utils.Logger.Printf("Failed to store verification code: %v", err)
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to store verification code",
		})
		return
	}

	// Show success page with verification code
	c.HTML(http.StatusOK, "success.html", gin.H{
		"code":  verificationCode.Code,
		"email": userInfo.Email,
	})
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

func (h *OAuthHandler) exchangeCodeForToken(code string) (*tokenResponse, error) {
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", h.config.MicrosoftTenantID)
	
	data := url.Values{}
	data.Set("client_id", h.config.MicrosoftClientID)
	data.Set("client_secret", h.config.MicrosoftClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", h.config.MicrosoftRedirectURL)
	data.Set("grant_type", "authorization_code")

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed: %s", string(body))
	}

	var tokenResp tokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

type userInfo struct {
	Email string `json:"mail"`
	Name  string `json:"displayName"`
}

func (h *OAuthHandler) getUserInfo(accessToken string) (*userInfo, error) {
	req, err := http.NewRequest("GET", "https://graph.microsoft.com/v1.0/me", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get user info: %s", string(body))
	}

	var info userInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}

	return &info, nil
}