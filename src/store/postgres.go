package store

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore holds the connection pool.
type PostgresStore struct {
	db *pgxpool.Pool
}

// NewPostgresStore creates a new store, connects to the DB, and initializes the schema.
func NewPostgresStore(databaseURL string, schemaPath string) (*PostgresStore, error) {
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %v", err)
	}

	// Verify the connection
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database ping failed: %v", err)
	}

	// Initialize the schema (from schema.sql)
	schemaSQL, err := os.ReadFile(schemaPath)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("could not read schema file: %v", err)
	}

	if _, err := pool.Exec(context.Background(), string(schemaSQL)); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to apply schema: %v", err)
	}

	return &PostgresStore{db: pool}, nil
}

// Close closes the database connection pool.
func (s *PostgresStore) Close() {
	s.db.Close()
}

// ---- User Methods ----

// RegisterUser is the Go equivalent of the INSERT query in your /register endpoint.
func (s *PostgresStore) RegisterUser(ctx context.Context, username string, passwordHash string) error {
	// db.Exec is for queries that don't return rows.
	_, err := s.db.Exec(ctx,
		"INSERT INTO users (username, password_hash) VALUES ($1, $2)",
		username, passwordHash)

	if err != nil {
		// This is how you check for a "UniqueViolation" in pgx
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			return fmt.Errorf("username already exists")
		}
		return fmt.Errorf("database error: %v", err)
	}

	return nil
}

// TODO: Add more methods here to match server.py. For example:
// func (s *PostgresStore) GetUserByUsername(ctx context.Context, username string) (*User, error)
// func (s *PostgresStore) UploadPublicKey(ctx context.Context, userID int, key string) error
// func (s *PostgresStore) GetPublicKey(ctx context.Context, username string) (string, error)
// ... and so on for chat requests, messages, etc.
