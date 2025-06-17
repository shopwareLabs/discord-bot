package models

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Database struct {
	db *sql.DB
}

func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	database := &Database{db: db}

	if err := database.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	go database.cleanup()

	return database, nil
}

func (d *Database) migrate() error {
	usersTable := `
	CREATE TABLE IF NOT EXISTS users (
		user_id INTEGER PRIMARY KEY AUTOINCREMENT,
		discord_id TEXT UNIQUE NOT NULL,
		azure_user_id TEXT UNIQUE NOT NULL,
		email TEXT NOT NULL,
		name TEXT NOT NULL,
		verified_at DATETIME NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`

	verificationsTable := `
	CREATE TABLE IF NOT EXISTS verifications (
		user_id INTEGER PRIMARY KEY AUTOINCREMENT,
		code TEXT UNIQUE NOT NULL,
		discord_id TEXT NOT NULL,
		email TEXT NOT NULL,
		expires_at DATETIME NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`

	addAzureUserIDColumn := `ALTER TABLE users ADD COLUMN azure_user_id TEXT;`

	indexDiscordID := `CREATE INDEX IF NOT EXISTS idx_users_discord_id ON users(discord_id);`
	indexAzureUserID := `CREATE INDEX IF NOT EXISTS idx_users_azure_user_id ON users(azure_user_id);`
	indexVerificationCode := `CREATE INDEX IF NOT EXISTS idx_verifications_code ON verifications(code);`
	indexVerificationDiscordID := `CREATE INDEX IF NOT EXISTS idx_verifications_discord_id ON verifications(discord_id);`
	indexVerificationExpires := `CREATE INDEX IF NOT EXISTS idx_verifications_expires_at ON verifications(expires_at);`

	queries := []string{
		usersTable,
		verificationsTable,
		indexDiscordID,
		indexAzureUserID,
		indexVerificationCode,
		indexVerificationDiscordID,
		indexVerificationExpires,
	}

	migrationQueries := []string{
		addAzureUserIDColumn,
	}

	for _, query := range migrationQueries {
		_, _ = d.db.Exec(query)
	}

	for _, query := range queries {
		if _, err := d.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute migration query: %w", err)
		}
	}

	return nil
}

func (d *Database) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		_, err := d.db.Exec("DELETE FROM verifications WHERE expires_at < ?", time.Now())
		if err != nil {
			// Log error but don't stop the cleanup process
			fmt.Printf("Failed to cleanup expired verifications: %v\n", err)
		}
	}
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) GetDB() *sql.DB {
	return d.db
}
