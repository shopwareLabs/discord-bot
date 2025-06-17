package models

import (
	"database/sql"
	"fmt"
	"time"
)

type VerificationCode struct {
	Code      string
	Email     string
	DiscordID string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type User struct {
	UserID      int
	DiscordID   string
	AzureUserID string
	Email       string
	Name        string
	VerifiedAt  time.Time
	CreatedAt   time.Time
}

type VerificationStore struct {
	db *Database
}

func NewVerificationStore(db *Database) *VerificationStore {
	return &VerificationStore{
		db: db,
	}
}

func (s *VerificationStore) Store(code *VerificationCode) error {
	query := `
		INSERT INTO verifications (code, discord_id, email, expires_at)
		VALUES (?, ?, ?, ?)
	`

	_, err := s.db.GetDB().Exec(query, code.Code, code.DiscordID, code.Email, code.ExpiresAt)
	if err != nil {
		return fmt.Errorf("failed to store verification code: %w", err)
	}

	return nil
}

func (s *VerificationStore) Get(code string) (*VerificationCode, bool) {
	query := `
		SELECT code, discord_id, email, expires_at, created_at
		FROM verifications
		WHERE code = ? AND expires_at > ?
	`

	row := s.db.GetDB().QueryRow(query, code, time.Now())

	var vc VerificationCode
	err := row.Scan(&vc.Code, &vc.DiscordID, &vc.Email, &vc.ExpiresAt, &vc.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false
		}
		// Log error but return false
		fmt.Printf("Error getting verification code: %v\n", err)
		return nil, false
	}

	return &vc, true
}

func (s *VerificationStore) GetByDiscordID(discordID string) (*VerificationCode, bool) {
	query := `
		SELECT code, discord_id, email, expires_at, created_at
		FROM verifications
		WHERE discord_id = ? AND expires_at > ?
		ORDER BY created_at DESC
		LIMIT 1
	`

	row := s.db.GetDB().QueryRow(query, discordID, time.Now())

	var vc VerificationCode
	err := row.Scan(&vc.Code, &vc.DiscordID, &vc.Email, &vc.ExpiresAt, &vc.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false
		}
		fmt.Printf("Error getting verification code by Discord ID: %v\n", err)
		return nil, false
	}

	return &vc, true
}

func (s *VerificationStore) Delete(code string) error {
	query := `DELETE FROM verifications WHERE code = ?`

	_, err := s.db.GetDB().Exec(query, code)
	if err != nil {
		return fmt.Errorf("failed to delete verification code: %w", err)
	}

	return nil
}

func (s *VerificationStore) CreateUser(discordID, email, name string) error {
	query := `
		INSERT INTO users (discord_id, email, name, verified_at)
		VALUES (?, ?, ?, ?)
	`

	_, err := s.db.GetDB().Exec(query, discordID, email, name, time.Now())
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

func (s *VerificationStore) CreateUserWithAzureID(discordID, azureUserID, email, name string) error {
	query := `
		INSERT INTO users (discord_id, azure_user_id, email, name, verified_at)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err := s.db.GetDB().Exec(query, discordID, azureUserID, email, name, time.Now())
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

func (s *VerificationStore) GetUser(discordID string) (*User, bool) {
	query := `
		SELECT user_id, discord_id, COALESCE(azure_user_id, '') as azure_user_id, email, name, verified_at, created_at
		FROM users
		WHERE discord_id = ?
	`

	row := s.db.GetDB().QueryRow(query, discordID)

	var user User
	err := row.Scan(&user.UserID, &user.DiscordID, &user.AzureUserID, &user.Email, &user.Name, &user.VerifiedAt, &user.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false
		}
		fmt.Printf("Error getting user: %v\n", err)
		return nil, false
	}

	return &user, true
}

func (s *VerificationStore) GetUserByAzureID(azureUserID string) (*User, bool) {
	query := `
		SELECT user_id, discord_id, azure_user_id, email, name, verified_at, created_at
		FROM users
		WHERE azure_user_id = ?
	`

	row := s.db.GetDB().QueryRow(query, azureUserID)

	var user User
	err := row.Scan(&user.UserID, &user.DiscordID, &user.AzureUserID, &user.Email, &user.Name, &user.VerifiedAt, &user.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false
		}
		// Log error but return false
		fmt.Printf("Error getting user by Azure ID: %v\n", err)
		return nil, false
	}

	return &user, true
}

func (s *VerificationStore) IsUserVerifiedByAzureID(azureUserID string) bool {
	_, exists := s.GetUserByAzureID(azureUserID)
	return exists
}

func (s *VerificationStore) IsUserVerified(discordID string) bool {
	_, exists := s.GetUser(discordID)
	return exists
}
