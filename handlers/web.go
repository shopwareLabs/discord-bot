package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type WebHandler struct {
	discordHandler *DiscordHandler
}

func NewWebHandler(discordHandler *DiscordHandler) *WebHandler {
	return &WebHandler{
		discordHandler: discordHandler,
	}
}

// Home shows the home page
func (h *WebHandler) Home(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title": "Discord Employee Verification",
	})
}

// VerifyCode handles the verification code submission
func (h *WebHandler) VerifyCode(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Verify the user
	if err := h.discordHandler.VerifyUser(req.Code); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Verification successful! Check Discord for confirmation."})
}