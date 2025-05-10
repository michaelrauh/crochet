package types

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3" // Import SQLite driver
	"github.com/tursodatabase/libsql-client-go/libsql"
)

// ContextStore defines the interface for context storage
type ContextStore interface {
	SaveVocabulary(vocabulary []string) []string
	SaveSubphrases(subphrases [][]string) [][]string
	SaveOrthos(orthos []Ortho) error // Add this method
	GetVocabulary() []string
	GetSubphrases() [][]string
	GetVersion() (int, error) // Modified to return error
	SetVersion(version int) error
	GetOrthoCount() (int, error)
	GetOrthoCountByShapePosition() (map[string]map[string]int, error)
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
	fmt.Println("Starting schema initialization...")

	// Create vocabulary table
	_, err := ls.db.Exec(`
		CREATE TABLE IF NOT EXISTS vocabulary (
			word TEXT PRIMARY KEY
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating vocabulary table: %w", err)
	}
	fmt.Println("Vocabulary table created or already exists")

	// Create subphrases table
	_, err = ls.db.Exec(`
		CREATE TABLE IF NOT EXISTS subphrases (
			phrase TEXT PRIMARY KEY
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating subphrases table: %w", err)
	}
	fmt.Println("Subphrases table created or already exists")

	// Create orthos table
	_, err = ls.db.Exec(`
		CREATE TABLE IF NOT EXISTS orthos (
			id TEXT PRIMARY KEY,
			grid TEXT NOT NULL,
			shape TEXT NOT NULL,
			position TEXT NOT NULL,
			shell INTEGER NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating orthos table: %w", err)
	}
	fmt.Println("Orthos table created or already exists")

	// Create remediations table - ensuring it supports arbitrary-length phrases via pair_key
	_, err = ls.db.Exec(`
		CREATE TABLE IF NOT EXISTS remediations (
			id TEXT PRIMARY KEY,
			ortho_id TEXT NOT NULL,
			pair_key TEXT NOT NULL,
			FOREIGN KEY (ortho_id) REFERENCES orthos(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating remediations table: %w", err)
	}
	fmt.Println("Remediations table created or already exists")

	// Create index on pair_key for efficient lookups of arbitrary-length phrases
	_, err = ls.db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_remediations_pair_key ON remediations(pair_key)
	`)
	if err != nil {
		return fmt.Errorf("error creating index on remediations pair_key: %w", err)
	}
	fmt.Println("Remediations index created or already exists")

	// Create version table
	_, err = ls.db.Exec(`
		CREATE TABLE IF NOT EXISTS version (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			version INTEGER NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating version table: %w", err)
	}
	fmt.Println("Version table created or already exists")

	// Initialize version if not exists
	_, err = ls.db.Exec(`
		INSERT OR IGNORE INTO version (id, version) VALUES (1, 1)
	`)
	if err != nil {
		return fmt.Errorf("error initializing version: %w", err)
	}
	fmt.Println("Version initialized if needed")

	// Verify tables exist
	tables := []string{"vocabulary", "subphrases", "orthos", "remediations", "version"}
	for _, table := range tables {
		var count int
		err = ls.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count)
		if err != nil {
			return fmt.Errorf("error verifying table %s: %w", table, err)
		}
		if count == 0 {
			return fmt.Errorf("table %s was not created properly", table)
		}
		fmt.Printf("Verified table %s exists\n", table)
	}

	fmt.Println("Schema initialization completed successfully")
	return nil
}

// DB returns the underlying database connection
func (ls *LibSQLContextStore) DB() *sql.DB {
	return ls.db
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

// GetVersion retrieves the current version from the database
func (ls *LibSQLContextStore) GetVersion() (int, error) {
	var version int
	err := ls.db.QueryRow("SELECT version FROM version WHERE id = 1").Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("error reading version: %w", err)
	}
	return version, nil
}

// SetVersion updates the version in the database
func (ls *LibSQLContextStore) SetVersion(version int) error {
	_, err := ls.db.Exec("UPDATE version SET version = ? WHERE id = 1", version)
	if err != nil {
		return fmt.Errorf("error updating version: %w", err)
	}
	return nil
}

// GetOrthoCount returns the total count of orthos in the database
func (ls *LibSQLContextStore) GetOrthoCount() (int, error) {
	var count int
	err := ls.db.QueryRow("SELECT COUNT(*) FROM orthos").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("error counting orthos: %w", err)
	}
	return count, nil
}

// GetOrthoCountByShapePosition returns the count of orthos grouped by shape and position
func (ls *LibSQLContextStore) GetOrthoCountByShapePosition() (map[string]map[string]int, error) {
	rows, err := ls.db.Query("SELECT shape, position, COUNT(*) FROM orthos GROUP BY shape, position")
	if err != nil {
		return nil, fmt.Errorf("error querying orthos by shape and position: %w", err)
	}
	defer rows.Close()

	result := make(map[string]map[string]int)
	for rows.Next() {
		var shape, position string
		var count int
		if err := rows.Scan(&shape, &position, &count); err != nil {
			return nil, fmt.Errorf("error scanning ortho count row: %w", err)
		}

		if _, ok := result[shape]; !ok {
			result[shape] = make(map[string]int)
		}
		result[shape][position] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ortho count rows: %w", err)
	}

	return result, nil
}

// SaveOrthos saves orthos to the database
func (ls *LibSQLContextStore) SaveOrthos(orthos []Ortho) error {
	tx, err := ls.db.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	stmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO orthos (id, grid, shape, position, shell)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("error preparing statement: %w", err)
	}
	defer stmt.Close()

	for _, ortho := range orthos {
		// Serialize the grid map to JSON
		gridJSON, err := json.Marshal(ortho.Grid)
		if err != nil {
			return fmt.Errorf("error marshaling grid to JSON: %w", err)
		}

		// Serialize the shape and position slices to JSON
		shapeJSON, err := json.Marshal(ortho.Shape)
		if err != nil {
			return fmt.Errorf("error marshaling shape to JSON: %w", err)
		}

		positionJSON, err := json.Marshal(ortho.Position)
		if err != nil {
			return fmt.Errorf("error marshaling position to JSON: %w", err)
		}

		_, err = stmt.Exec(
			ortho.ID,
			string(gridJSON),
			string(shapeJSON),
			string(positionJSON),
			ortho.Shell,
		)
		if err != nil {
			return fmt.Errorf("error inserting ortho: %w", err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}

	return nil
}

// Close closes the database connection
func (ls *LibSQLContextStore) Close() error {
	return ls.db.Close()
}

// SplitSubphrase splits a joined subphrase string into a slice of strings
func SplitSubphrase(joinedSubphrase string) []string {
	return strings.Split(joinedSubphrase, " ")
}
