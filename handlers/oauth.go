package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"discord-sso-role/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"
)

type OAuthHandler struct {
	config      *models.Config
	store       *models.VerificationStore
	oauthConfig *oauth2.Config
}

func NewOAuthHandler(config *models.Config, store *models.VerificationStore) *OAuthHandler {
	oauthConfig := &oauth2.Config{
		ClientID:     config.MicrosoftClientID,
		ClientSecret: config.MicrosoftClientSecret,
		RedirectURL:  config.MicrosoftRedirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     microsoft.AzureADEndpoint(config.MicrosoftTenantID),
	}

	return &OAuthHandler{
		config:      config,
		store:       store,
		oauthConfig: oauthConfig,
	}
}

// StartAuth initiates the OAuth flow
func (h *OAuthHandler) StartAuth(c *gin.Context) {
	state := c.Query("state") // Discord ID passed as state
	if state == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing state parameter"})
		return
	}

	authURL := h.oauthConfig.AuthCodeURL(state)
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
	ctx := context.Background()
	token, err := h.oauthConfig.Exchange(ctx, code)
	if err != nil {
		slog.Error("Failed to exchange code for token", "error", err)
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to authenticate with Microsoft",
		})
		return
	}

	// Get user info using the OAuth2 client
	client := h.oauthConfig.Client(ctx, token)
	resp, err := client.Get("https://graph.microsoft.com/v1.0/me")
	if err != nil {
		slog.Error("Failed to get user info", "error", err)
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to get user information",
		})
		return
	}
	defer resp.Body.Close()

	var userInfo struct {
		Email string `json:"mail"`
		Name  string `json:"displayName"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		slog.Error("Failed to decode user info", "error", err)
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to parse user information",
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
		slog.Error("Failed to store verification code", "error", err)
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
