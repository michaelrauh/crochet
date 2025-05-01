package types

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3" // Import SQLite driver
	"github.com/tursodatabase/libsql-client-go/libsql"
)

// ContextStore defines the interface for context storage
type ContextStore interface {
	SaveVocabulary(vocabulary []string) []string
	SaveSubphrases(subphrases [][]string) [][]string
	GetVocabulary() []string
	GetSubphrases() [][]string
	Close() error
}

// LibSQLContextStore is the SQLite implementation of ContextStore
type LibSQLContextStore struct {
	db *sql.DB
}

// NewLibSQLContextStore creates a new SQLite-backed context store
func NewLibSQLContextStore(connURL string) (*LibSQLContextStore, error) {
	var db *sql.DB
	var err error

	// Check if we have embedded replica format (local_path|remote_url|auth_token)
	if strings.Contains(connURL, "|") {
		parts := strings.Split(connURL, "|")
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid connection URL format; expected 'dbPath|primaryUrl|authToken'")
		}

		dbPath := parts[0]
		remoteURL := parts[1]
		authToken := parts[2]

		// Configure options based on provided values
		var opts []libsql.Option

		// Set the remote URL as proxy if provided
		if remoteURL != "" && remoteURL != "none" {
			opts = append(opts, libsql.WithProxy(remoteURL))
		}

		// Add auth token if provided and not 'none'
		if authToken != "" && authToken != "none" {
			opts = append(opts, libsql.WithAuthToken(authToken))
		}

		// Create the connector with the options
		connector, err := libsql.NewConnector(dbPath, opts...)
		if err != nil {
			return nil, fmt.Errorf("error creating libsql connector: %w", err)
		}

		// Create database connection using the connector
		db = sql.OpenDB(connector)
	} else {
		// Regular SQLite connection for local-only database
		db, err = sql.Open("sqlite3", connURL)
		if err != nil {
			return nil, fmt.Errorf("error opening sqlite connection: %w", err)
		}
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("error connecting to database: %w", err)
	}

	store := &LibSQLContextStore{
		db: db,
	}

	// Initialize the database schema
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("error initializing schema: %w", err)
	}

	return store, nil
}

// initSchema creates the necessary tables if they don't exist
func (ls *LibSQLContextStore) initSchema() error {
	// Create vocabulary table
	_, err := ls.db.Exec(`
		CREATE TABLE IF NOT EXISTS vocabulary (
			word TEXT PRIMARY KEY
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating vocabulary table: %w", err)
	}

	// Create subphrases table
	_, err = ls.db.Exec(`
		CREATE TABLE IF NOT EXISTS subphrases (
			phrase TEXT PRIMARY KEY
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating subphrases table: %w", err)
	}

	return nil
}

// SaveVocabulary adds new vocabulary to the store and returns newly added items
func (ls *LibSQLContextStore) SaveVocabulary(vocabulary []string) []string {
	if len(vocabulary) == 0 {
		return nil
	}

	ctx := context.Background()
	tx, err := ls.db.BeginTx(ctx, nil)
	if err != nil {
		// Log error and return empty slice
		fmt.Printf("Error starting transaction: %v\n", err)
		return nil
	}
	defer tx.Rollback()

	// Prepare the statement for checking existence
	checkStmt, err := tx.PrepareContext(ctx, "SELECT 1 FROM vocabulary WHERE word = ?")
	if err != nil {
		fmt.Printf("Error preparing check statement: %v\n", err)
		return nil
	}
	defer checkStmt.Close()

	// Prepare the statement for insertion
	insertStmt, err := tx.PrepareContext(ctx, "INSERT INTO vocabulary (word) VALUES (?)")
	if err != nil {
		fmt.Printf("Error preparing insert statement: %v\n", err)
		return nil
	}
	defer insertStmt.Close()

	var newlyAdded []string
	for _, word := range vocabulary {
		// Check if word already exists
		var exists bool
		err := checkStmt.QueryRowContext(ctx, word).Scan(&exists)
		if err != nil && err != sql.ErrNoRows {
			fmt.Printf("Error checking word existence: %v\n", err)
			continue
		}

		// If not exists (got ErrNoRows), insert it
		if err == sql.ErrNoRows {
			_, err = insertStmt.ExecContext(ctx, word)
			if err != nil {
				fmt.Printf("Error inserting word: %v\n", err)
				continue
			}
			newlyAdded = append(newlyAdded, word)
		}
	}

	if err := tx.Commit(); err != nil {
		fmt.Printf("Error committing transaction: %v\n", err)
		return nil
	}

	return newlyAdded
}

// SaveSubphrases adds new subphrases to the store and returns newly added items
func (ls *LibSQLContextStore) SaveSubphrases(subphrases [][]string) [][]string {
	if len(subphrases) == 0 {
		return nil
	}

	ctx := context.Background()
	tx, err := ls.db.BeginTx(ctx, nil)
	if err != nil {
		fmt.Printf("Error starting transaction: %v\n", err)
		return nil
	}
	defer tx.Rollback()

	// Prepare the statement for checking existence
	checkStmt, err := tx.PrepareContext(ctx, "SELECT 1 FROM subphrases WHERE phrase = ?")
	if err != nil {
		fmt.Printf("Error preparing check statement: %v\n", err)
		return nil
	}
	defer checkStmt.Close()

	// Prepare the statement for insertion
	insertStmt, err := tx.PrepareContext(ctx, "INSERT INTO subphrases (phrase) VALUES (?)")
	if err != nil {
		fmt.Printf("Error preparing insert statement: %v\n", err)
		return nil
	}
	defer insertStmt.Close()

	var newlyAdded [][]string
	for _, subphrase := range subphrases {
		joinedSubphrase := strings.Join(subphrase, " ")

		// Check if subphrase already exists
		var exists bool
		err := checkStmt.QueryRowContext(ctx, joinedSubphrase).Scan(&exists)
		if err != nil && err != sql.ErrNoRows {
			fmt.Printf("Error checking subphrase existence: %v\n", err)
			continue
		}

		// If not exists (got ErrNoRows), insert it
		if err == sql.ErrNoRows {
			_, err = insertStmt.ExecContext(ctx, joinedSubphrase)
			if err != nil {
				fmt.Printf("Error inserting subphrase: %v\n", err)
				continue
			}
			newlyAdded = append(newlyAdded, subphrase)
		}
	}

	if err := tx.Commit(); err != nil {
		fmt.Printf("Error committing transaction: %v\n", err)
		return nil
	}

	return newlyAdded
}

// GetVocabulary returns all vocabulary in the store
func (ls *LibSQLContextStore) GetVocabulary() []string {
	var vocabulary []string

	rows, err := ls.db.Query("SELECT word FROM vocabulary")
	if err != nil {
		fmt.Printf("Error querying vocabulary: %v\n", err)
		return vocabulary
	}
	defer rows.Close()

	for rows.Next() {
		var word string
		if err := rows.Scan(&word); err != nil {
			fmt.Printf("Error scanning vocabulary row: %v\n", err)
			continue
		}
		vocabulary = append(vocabulary, word)
	}

	if err := rows.Err(); err != nil {
		fmt.Printf("Error iterating vocabulary rows: %v\n", err)
	}

	return vocabulary
}

// GetSubphrases returns all subphrases in the store
func (ls *LibSQLContextStore) GetSubphrases() [][]string {
	var subphrases [][]string

	rows, err := ls.db.Query("SELECT phrase FROM subphrases")
	if err != nil {
		fmt.Printf("Error querying subphrases: %v\n", err)
		return subphrases
	}
	defer rows.Close()

	for rows.Next() {
		var phrase string
		if err := rows.Scan(&phrase); err != nil {
			fmt.Printf("Error scanning subphrase row: %v\n", err)
			continue
		}
		subphrases = append(subphrases, SplitSubphrase(phrase))
	}

	if err := rows.Err(); err != nil {
		fmt.Printf("Error iterating subphrase rows: %v\n", err)
	}

	return subphrases
}

// Close closes the database connection
func (ls *LibSQLContextStore) Close() error {
	return ls.db.Close()
}

// SplitSubphrase splits a joined subphrase string into a slice of strings
func SplitSubphrase(joinedSubphrase string) []string {
	return strings.Split(joinedSubphrase, " ")
}
