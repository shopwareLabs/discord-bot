package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"log/slog"
	"net/http"

	"discord-sso-role/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"
)

type OAuthHandler struct {
	config         *models.Config
	store          *models.VerificationStore
	oauthConfig    *oauth2.Config
	discordHandler *DiscordHandler
}

func NewOAuthHandler(config *models.Config, store *models.VerificationStore, discordHandler *DiscordHandler) *OAuthHandler {
	oauthConfig := &oauth2.Config{
		ClientID:     config.MicrosoftClientID,
		ClientSecret: config.MicrosoftClientSecret,
		RedirectURL:  config.MicrosoftRedirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     microsoft.AzureADEndpoint(config.MicrosoftTenantID),
	}

	return &OAuthHandler{
		config:         config,
		store:          store,
		oauthConfig:    oauthConfig,
		discordHandler: discordHandler,
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

	respContent, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Failed to read user info response", "error", err)
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to read user information",
		})
		return
	}

	log.Println(string(respContent))

	var userInfo struct {
		Email string `json:"email"`
		Name  string `json:"displayName"`
	}

	if err := json.NewDecoder(bytes.NewReader(respContent)).Decode(&userInfo); err != nil {
		slog.Error("Failed to decode user info", "error", err)
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to parse user information",
		})
		return
	}

	// Directly verify the user and assign the role
	err = h.discordHandler.VerifyUserDirectly(state, userInfo.Email)
	if err != nil {
		slog.Error("Failed to verify user", "error", err)
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": err.Error(),
		})
		return
	}

	// Show success page
	c.HTML(http.StatusOK, "success.html", gin.H{
		"email":   userInfo.Email,
		"message": "Your employee status has been verified! Check Discord for confirmation.",
	})
}
