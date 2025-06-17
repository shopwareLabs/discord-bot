package models

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds the application configuration
type Config struct {
	// Microsoft OAuth
	MicrosoftClientID     string
	MicrosoftClientSecret string
	MicrosoftRedirectURL  string
	MicrosoftTenantID     string

	// Discord
	DiscordToken   string
	DiscordGuildID string
	DiscordRoleID  string

	// Server
	Port            string
	BaseURL         string
	VerificationTTL int // in minutes

	// Database
	DatabasePath string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	ttl, _ := strconv.Atoi(getEnv("VERIFICATION_TTL", "15"))

	return &Config{
		MicrosoftClientID:     getEnv("MICROSOFT_CLIENT_ID", ""),
		MicrosoftClientSecret: getEnv("MICROSOFT_CLIENT_SECRET", ""),
		MicrosoftRedirectURL:  fmt.Sprintf("%s/employee/callback", getEnv("BASE_URL", "http://localhost:8080")),
		MicrosoftTenantID:     getEnv("MICROSOFT_TENANT_ID", ""),
		DiscordToken:          getEnv("DISCORD_TOKEN", ""),
		DiscordGuildID:        getEnv("DISCORD_GUILD_ID", ""),
		DiscordRoleID:         getEnv("DISCORD_ROLE_ID", ""),
		Port:                  getEnv("PORT", "8080"),
		BaseURL:               getEnv("BASE_URL", "http://localhost:8080"),
		VerificationTTL:       ttl,
		DatabasePath:          getEnv("DATABASE_PATH", "./data/discord-sso.db"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
