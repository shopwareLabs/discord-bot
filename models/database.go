package models

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Database holds the database connection and provides methods for database operations
type Database struct {
	db *sql.DB
}

// NewDatabase creates a new database connection and initializes the schema
func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	database := &Database{db: db}

	// Initialize schema
	if err := database.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	// Start cleanup goroutine
	go database.cleanup()

	return database, nil
}

// migrate creates the database schema
func (d *Database) migrate() error {
	// Create users table
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

	// Create verifications table
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

	// Add azure_user_id column if it doesn't exist (for existing databases)
	addAzureUserIDColumn := `ALTER TABLE users ADD COLUMN azure_user_id TEXT;`
	
	// Create indexes
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

	// Try to add the azure_user_id column for existing databases (will fail silently if column exists)
	migrationQueries := []string{
		addAzureUserIDColumn,
	}

	// Execute migration queries (may fail silently for existing columns)
	for _, query := range migrationQueries {
		d.db.Exec(query) // Ignore errors for existing columns
	}

	for _, query := range queries {
		if _, err := d.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute migration query: %w", err)
		}
	}

	return nil
}

// cleanup runs periodically to remove expired verification codes
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

// Close closes the database connection
func (d *Database) Close() error {
	return d.db.Close()
}

// GetDB returns the underlying database connection
func (d *Database) GetDB() *sql.DB {
	return d.db
}