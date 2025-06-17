package handlers

import (
	"fmt"
	"strings"

	"discord-sso-role/models"
	"discord-sso-role/utils"

	"github.com/bwmarrin/discordgo"
)

type DiscordHandler struct {
	session *discordgo.Session
	config  *models.Config
	store   *models.VerificationStore
}

func NewDiscordHandler(config *models.Config, store *models.VerificationStore) (*DiscordHandler, error) {
	dg, err := discordgo.New("Bot " + config.DiscordToken)
	if err != nil {
		return nil, err
	}

	handler := &DiscordHandler{
		session: dg,
		config:  config,
		store:   store,
	}

	// Register handlers
	dg.AddHandler(handler.ready)
	dg.AddHandler(handler.interactionCreate)

	return handler, nil
}

func (h *DiscordHandler) Start() error {
	err := h.session.Open()
	if err != nil {
		return err
	}

	// Register slash commands
	_, err = h.session.ApplicationCommandCreate(h.session.State.User.ID, h.config.DiscordGuildID, &discordgo.ApplicationCommand{
		Name:        "verify-employee",
		Description: "Verify your employee status to get the employee role",
	})
	if err != nil {
		return fmt.Errorf("cannot create slash command: %v", err)
	}

	utils.Logger.Println("Discord bot is now running")
	return nil
}

func (h *DiscordHandler) Stop() error {
	return h.session.Close()
}

func (h *DiscordHandler) ready(s *discordgo.Session, event *discordgo.Ready) {
	utils.Logger.Printf("Discord bot logged in as %s", event.User.Username)
}

func (h *DiscordHandler) interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.ApplicationCommandData().Name == "verify-employee" {
		h.handleVerifyCommand(s, i)
	}
}

func (h *DiscordHandler) handleVerifyCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Check if user already has a pending verification
	if existingCode, exists := h.store.GetByDiscordID(i.Member.User.ID); exists {
		// Send ephemeral message with existing verification link
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("You already have a pending verification. Your code is: **%s**\n\nPlease complete the verification process or wait for it to expire.", existingCode.Code),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			utils.Logger.Printf("Failed to respond to interaction: %v", err)
		}
		return
	}

	// Generate verification URL
	verificationURL := fmt.Sprintf("%s/auth/start?state=%s", h.config.BaseURL, i.Member.User.ID)

	// Send ephemeral message with verification link
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Please click the following link to verify your employee status:\n%s\n\nThis link will expire in %d minutes.", verificationURL, h.config.VerificationTTL),
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		utils.Logger.Printf("Failed to respond to interaction: %v", err)
	}
}

// VerifyUser verifies a user with the provided code and assigns the role
func (h *DiscordHandler) VerifyUser(code string) error {
	// Get verification code
	vc, exists := h.store.Get(code)
	if !exists {
		return fmt.Errorf("invalid or expired verification code")
	}

	// Check if email is from allowed domain (you can customize this)
	if !strings.HasSuffix(vc.Email, "@shopware.com") {
		if err := h.store.Delete(code); err != nil {
			utils.Logger.Printf("Failed to delete verification code: %v", err)
		}
		return fmt.Errorf("email domain not allowed")
	}

	// Check if user is already verified
	if h.store.IsUserVerified(vc.DiscordID) {
		if err := h.store.Delete(code); err != nil {
			utils.Logger.Printf("Failed to delete verification code: %v", err)
		}
		return fmt.Errorf("user is already verified")
	}

	// Add role to user
	err := h.session.GuildMemberRoleAdd(h.config.DiscordGuildID, vc.DiscordID, h.config.DiscordRoleID)
	if err != nil {
		return fmt.Errorf("failed to add role: %v", err)
	}

	// Get user info from Discord to store name
	user, err := h.session.User(vc.DiscordID)
	var userName string
	if err != nil {
		userName = "Unknown"
		utils.Logger.Printf("Failed to get Discord user info: %v", err)
	} else {
		userName = user.Username
	}

	// Create user record in database
	if err := h.store.CreateUser(vc.DiscordID, vc.Email, userName); err != nil {
		utils.Logger.Printf("Failed to create user record: %v", err)
		// Don't return error here as the role was already assigned
	}

	// Send DM to user
	channel, err := h.session.UserChannelCreate(vc.DiscordID)
	if err == nil {
		_, _ = h.session.ChannelMessageSend(channel.ID, fmt.Sprintf("Congratulations! Your employee status has been verified. Email: %s", vc.Email))
	}

	// Delete verification code after successful verification
	if err := h.store.Delete(code); err != nil {
		utils.Logger.Printf("Failed to delete verification code: %v", err)
	}

	utils.Logger.Printf("User %s verified with email %s", vc.DiscordID, vc.Email)
	return nil
}
