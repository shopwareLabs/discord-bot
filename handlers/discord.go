package handlers

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/shopwarelabs/discord-bot/models"

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

	dg.AddHandler(handler.ready)
	dg.AddHandler(handler.interactionCreate)

	return handler, nil
}

func (h *DiscordHandler) Start() error {
	err := h.session.Open()
	if err != nil {
		return err
	}

	_, err = h.session.ApplicationCommandCreate(h.session.State.User.ID, h.config.DiscordGuildID, &discordgo.ApplicationCommand{
		Name:        "verify-employee",
		Description: "Verify your employee status to get the employee role",
	})
	if err != nil {
		return fmt.Errorf("cannot create slash command: %v", err)
	}

	slog.Info("Discord bot started", "guild_id", h.config.DiscordGuildID)
	return nil
}

func (h *DiscordHandler) Stop() error {
	return h.session.Close()
}

func (h *DiscordHandler) ready(s *discordgo.Session, event *discordgo.Ready) {
	slog.Info("Discord bot logged in", "user", event.User.Username)
}

func (h *DiscordHandler) interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.ApplicationCommandData().Name == "verify-employee" {
		h.handleVerifyCommand(s, i)
	}
}

func (h *DiscordHandler) handleVerifyCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if h.store.IsUserVerified(i.Member.User.ID) {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You are already verified!",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			slog.Error("Failed to respond to interaction", "error", err)
		}
		return
	}

	verificationURL := fmt.Sprintf("%s/employee/start?state=%s", h.config.BaseURL, i.Member.User.ID)

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Please click the following link to verify your employee status:\n%s", verificationURL),
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		slog.Error("Failed to respond to interaction", "error", err)
	}
}

// VerifyUserDirectly verifies a user directly with Azure ID and email and assigns the role
func (h *DiscordHandler) VerifyUserDirectly(discordID, azureUserID, email string) error {
	if !strings.HasSuffix(email, "@shopware.com") {
		return fmt.Errorf("email domain not allowed")
	}

	if h.store.IsUserVerifiedByAzureID(azureUserID) {
		return fmt.Errorf("user is already verified")
	}

	slog.Info("Assigning role to user", "discord_id", discordID, "azure_id", azureUserID, "guild_id", h.config.DiscordGuildID, "role_id", h.config.DiscordRoleID)
	err := h.session.GuildMemberRoleAdd(h.config.DiscordGuildID, discordID, h.config.DiscordRoleID)
	if err != nil {
		return fmt.Errorf("failed to add role: %v", err)
	}

	user, err := h.session.User(discordID)
	var userName string
	if err != nil {
		return fmt.Errorf("failed to get Discord user info: %v", err)
	} else {
		userName = user.Username
	}

	if err := h.store.CreateUserWithAzureID(discordID, azureUserID, email, userName); err != nil {
		return fmt.Errorf("failed to create user record: %v", err)
	}

	channel, err := h.session.UserChannelCreate(discordID)
	if err == nil {
		_, _ = h.session.ChannelMessageSend(channel.ID, fmt.Sprintf("Congratulations! Your employee status has been verified. Email: %s", email))
	}

	slog.Info("User verified", "discord_id", discordID, "azure_id", azureUserID, "email", email)
	return nil
}
