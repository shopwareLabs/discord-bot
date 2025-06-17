package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"discord-sso-role/handlers"
	"discord-sso-role/models"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if it exists
	_ = godotenv.Load()

	// Load configuration
	config := models.LoadConfig()

	// Validate required configuration
	if config.MicrosoftClientID == "" || config.MicrosoftClientSecret == "" ||
		config.DiscordToken == "" || config.DiscordGuildID == "" || config.DiscordRoleID == "" {
		slog.Error("Missing required configuration. Please check your environment variables.")
	}

	// Ensure database directory exists
	if err := os.MkdirAll(filepath.Dir(config.DatabasePath), 0755); err != nil {
		slog.Error("Failed to create database directory: %v", err)
	}

	// Initialize database
	db, err := models.NewDatabase(config.DatabasePath)
	if err != nil {
		slog.Error("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Create verification store
	store := models.NewVerificationStore(db)

	// Initialize handlers
	oauthHandler := handlers.NewOAuthHandler(config, store)

	discordHandler, err := handlers.NewDiscordHandler(config, store)
	if err != nil {
		slog.Error("Failed to create Discord handler: %v", err)
	}

	webHandler := handlers.NewWebHandler(discordHandler)

	// Start Discord bot
	if err := discordHandler.Start(); err != nil {
		slog.Error("Failed to start Discord bot: %v", err)
	}
	defer discordHandler.Stop()

	// Setup Gin router
	router := gin.Default()
	router.LoadHTMLGlob("templates/*")

	// Routes
	router.GET("/", webHandler.Home)
	router.GET("/employee/start", oauthHandler.StartAuth)
	router.GET("/employee/callback", oauthHandler.Callback)
	router.POST("/employee/verify", webHandler.VerifyCode)

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// Create HTTP server
	srv := &http.Server{
		Addr:    ":" + config.Port,
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		slog.Info("Starting web server on port", "port", config.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Failed to start server", "err", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown: %v", err)
	}

	slog.Info("Server exited")
}
