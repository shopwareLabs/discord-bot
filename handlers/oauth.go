package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"

	"discord-sso-role/models"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"
)

type OAuthHandler struct {
	config         *models.Config
	store          *models.VerificationStore
	oauthConfig    *oauth2.Config
	discordHandler *DiscordHandler
	verifier       *oidc.IDTokenVerifier
}

func NewOAuthHandler(config *models.Config, store *models.VerificationStore, discordHandler *DiscordHandler) (*OAuthHandler, error) {
	// Create OIDC provider
	ctx := context.Background()
	issuerURL := fmt.Sprintf("https://login.microsoftonline.com/%s/v2.0", config.MicrosoftTenantID)
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
	}

	oauthConfig := &oauth2.Config{
		ClientID:     config.MicrosoftClientID,
		ClientSecret: config.MicrosoftClientSecret,
		RedirectURL:  config.MicrosoftRedirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     microsoft.AzureADEndpoint(config.MicrosoftTenantID),
	}

	verifier := provider.Verifier(&oidc.Config{
		ClientID: config.MicrosoftClientID,
	})

	return &OAuthHandler{
		config:         config,
		store:          store,
		oauthConfig:    oauthConfig,
		discordHandler: discordHandler,
		verifier:       verifier,
	}, nil
}

// StartAuth initiates the OAuth flow
func (h *OAuthHandler) StartAuth(c *gin.Context) {
	discordID := c.Query("state") // Discord ID passed as state
	if discordID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing Discord ID"})
		return
	}

	// Generate a secure random state parameter
	state, err := generateSecureState()
	if err != nil {
		slog.Error("Failed to generate state", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate state"})
		return
	}

	// Store Discord ID in session with the state as key
	session := sessions.Default(c)
	session.Set("discord_id_"+state, discordID)
	session.Set("oauth_state", state)
	if err := session.Save(); err != nil {
		slog.Error("Failed to save session", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Session error"})
		return
	}

	// Redirect to OAuth provider with secure state
	authURL := h.oauthConfig.AuthCodeURL(state)
	c.Redirect(http.StatusTemporaryRedirect, authURL)
}

// generateSecureState generates a cryptographically secure random state parameter
func generateSecureState() (string, error) {
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// Callback handles the OAuth callback
func (h *OAuthHandler) Callback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": "Authorization code not provided",
		})
		return
	}

	if state == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": "Missing state parameter",
		})
		return
	}

	// Validate state and get Discord ID from session
	session := sessions.Default(c)
	sessionState := session.Get("oauth_state")
	if sessionState == nil || sessionState.(string) != state {
		slog.Error("Invalid state parameter", "received", state, "expected", sessionState)
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": "Invalid state parameter",
		})
		return
	}

	// Get Discord ID from session
	discordIDKey := "discord_id_" + state
	discordIDValue := session.Get(discordIDKey)
	if discordIDValue == nil {
		slog.Error("Discord ID not found in session", "state", state)
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": "Session expired or invalid",
		})
		return
	}

	discordID := discordIDValue.(string)

	// Clean up session
	session.Delete("oauth_state")
	session.Delete(discordIDKey)
	session.Save()

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

	// Extract the ID token
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		slog.Error("No ID token found in response")
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "No ID token found",
		})
		return
	}

	// Parse and verify the ID token
	if h.verifier == nil {
		slog.Error("OIDC verifier not initialized")
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Authentication configuration error",
		})
		return
	}

	idToken, err := h.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		slog.Error("Failed to verify ID token", "error", err)
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to verify ID token",
		})
		return
	}

	// Extract claims
	var claims struct {
		Sub               string `json:"sub"`
		Email             string `json:"email"`
		PreferredUsername string `json:"preferred_username"`
		UPN               string `json:"upn"`
	}
	if err := idToken.Claims(&claims); err != nil {
		slog.Error("Failed to parse ID token claims", "error", err)
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to parse ID token claims",
		})
		return
	}

	// Ensure we have the Azure user ID
	if claims.Sub == "" {
		slog.Error("No subject (user ID) found in ID token", "claims", claims)
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Invalid authentication response",
		})
		return
	}

	// Get email for display purposes (but use Sub for database)
	email := claims.Email
	if email == "" {
		email = claims.PreferredUsername
		if email == "" {
			email = claims.UPN
		}
	}

	if email == "" {
		slog.Error("No email found in ID token", "claims", claims)
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "No email found in authentication response",
		})
		return
	}

	// Directly verify the user and assign the role
	err = h.discordHandler.VerifyUserDirectly(discordID, claims.Sub, email)
	if err != nil {
		slog.Error("Failed to verify user", "error", err)
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": err.Error(),
		})
		return
	}

	// Show success page
	c.HTML(http.StatusOK, "success.html", gin.H{
		"email":   email,
		"message": "Your employee status has been verified! Check Discord for confirmation.",
	})
}

