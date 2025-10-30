package store

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore holds the connection pool.
type PostgresStore struct {
	db *pgxpool.Pool
}

// User struct to hold user data
type User struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"` // Omit from JSON responses
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

// checkUniqueViolation is a helper to check for pgx "unique_violation" errors
func isUniqueViolation(err error) bool {
	if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
		return true
	}
	return false
}

// ---- User Methods ----

// RegisterUser is the Go equivalent of the INSERT query in your /register endpoint.
func (s *PostgresStore) RegisterUser(ctx context.Context, username string, passwordHash string) error {
	// db.Exec is for queries that don't return rows.
	_, err := s.db.Exec(ctx,
		"INSERT INTO users (username, password_hash) VALUES ($1, $2)",
		username, passwordHash)

	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("username already exists")
		}
		return fmt.Errorf("database error: %v", err)
	}

	return nil
}

// GetUserByUsername fetches a user for the login handler.
func (s *PostgresStore) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	var user User
	err := s.db.QueryRow(ctx,
		"SELECT id, username, password_hash FROM users WHERE username = $1",
		username,
	).Scan(&user.ID, &user.Username, &user.PasswordHash)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("database error: %v", err)
	}
	return &user, nil
}

// GetUserByID fetches a user for the auth middleware.
func (s *PostgresStore) GetUserByID(ctx context.Context, id int) (*User, error) {
	var user User
	err := s.db.QueryRow(ctx,
		"SELECT id, username, password_hash FROM users WHERE id = $1",
		id,
	).Scan(&user.ID, &user.Username, &user.PasswordHash)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("database error: %v", err)
	}
	return &user, nil
}

// GetUserIDByUsername is a helper to get just the ID for a given username.
func (s *PostgresStore) GetUserIDByUsername(ctx context.Context, username string) (int, error) {
	var id int
	err := s.db.QueryRow(ctx, "SELECT id FROM users WHERE username = $1", username).Scan(&id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, fmt.Errorf("user not found")
		}
		return 0, fmt.Errorf("database error: %v", err)
	}
	return id, nil
}

// ---- Key Methods ----

// UploadPublicKey upserts a user's public key.
func (s *PostgresStore) UploadPublicKey(ctx context.Context, userID int, key string) error {
	_, err := s.db.Exec(ctx,
		`
        INSERT INTO public_keys (user_id, public_key) VALUES ($1, $2)
        ON CONFLICT (user_id) DO UPDATE SET public_key = EXCLUDED.public_key
        `,
		userID, key)

	if err != nil {
		return fmt.Errorf("database error: %v", err)
	}
	return nil
}

// GetPublicKeyByUsername fetches a public key for a given username.
func (s *PostgresStore) GetPublicKeyByUsername(ctx context.Context, username string) (string, error) {
	var publicKey string
	err := s.db.QueryRow(ctx,
		`
        SELECT pk.public_key 
        FROM public_keys pk 
        JOIN users u ON u.id = pk.user_id 
        WHERE u.username = $1
        `,
		username,
	).Scan(&publicKey)

	if err != nil {
		if err == pgx.ErrNoRows {
			return "", fmt.Errorf("user not found or has no public key")
		}
		return "", fmt.Errorf("database error: %v", err)
	}
	return publicKey, nil
}

// ---- Chat Request Methods ----

// RequestChat creates a new 'pending' chat request.
func (s *PostgresStore) RequestChat(ctx context.Context, requesterID int, recipientUsername string) error {
	recipientID, err := s.GetUserIDByUsername(ctx, recipientUsername)
	if err != nil {
		return fmt.Errorf("recipient user not found")
	}

	if requesterID == recipientID {
		return fmt.Errorf("cannot send chat request to yourself")
	}

	_, err = s.db.Exec(ctx,
		"INSERT INTO chat_requests (requester_id, requested_id, status) VALUES ($1, $2, 'pending')",
		requesterID, recipientID,
	)

	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("chat request already pending or accepted")
		}
		return fmt.Errorf("database error: %v", err)
	}
	return nil
}

// PendingRequest struct for get_chat_requests response
type PendingRequest struct {
	RequesterUsername string `json:"requester_username"`
	Status            string `json:"status"`
}

// GetChatRequests fetches all pending requests for a user.
func (s *PostgresStore) GetChatRequests(ctx context.Context, requestedID int) ([]PendingRequest, error) {
	rows, err := s.db.Query(ctx,
		`
        SELECT u.username AS requester_username, cr.status
        FROM chat_requests cr
        JOIN users u ON u.id = cr.requester_id
        WHERE cr.requested_id = $1 AND cr.status = 'pending'
        `, requestedID)
	if err != nil {
		return nil, fmt.Errorf("database error: %v", err)
	}
	defer rows.Close()

	var requests []PendingRequest
	for rows.Next() {
		var req PendingRequest
		if err := rows.Scan(&req.RequesterUsername, &req.Status); err != nil {
			return nil, fmt.Errorf("database scan error: %v", err)
		}
		requests = append(requests, req)
	}
	return requests, nil
}

// AcceptChat updates a 'pending' request to 'accepted'.
func (s *PostgresStore) AcceptChat(ctx context.Context, requestedID int, requesterUsername string) error {
	requesterID, err := s.GetUserIDByUsername(ctx, requesterUsername)
	if err != nil {
		return fmt.Errorf("requester user not found")
	}

	cmdTag, err := s.db.Exec(ctx,
		`
        UPDATE chat_requests
        SET status = 'accepted'
        WHERE requester_id = $1 AND requested_id = $2 AND status = 'pending'
        `,
		requesterID, requestedID)

	if err != nil {
		return fmt.Errorf("database error: %v", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("no pending request found from that user")
	}
	return nil
}

// GetContacts fetches all accepted chat partners.
func (s *PostgresStore) GetContacts(ctx context.Context, myID int) ([]string, error) {
	contacts := make(map[string]struct{}) // Use a map as a set

	// 1. People I requested
	rows, err := s.db.Query(ctx,
		`
        SELECT u.username
        FROM chat_requests cr
        JOIN users u ON u.id = cr.requested_id
        WHERE cr.requester_id = $1 AND cr.status = 'accepted'
        `, myID)
	if err != nil {
		return nil, fmt.Errorf("database error (query 1): %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			return nil, fmt.Errorf("database scan error (query 1): %v", err)
		}
		contacts[username] = struct{}{}
	}

	// 2. People who requested me
	rows, err = s.db.Query(ctx,
		`
        SELECT u.username
        FROM chat_requests cr
        JOIN users u ON u.id = cr.requester_id
        WHERE cr.requested_id = $1 AND cr.status = 'accepted'
        `, myID)
	if err != nil {
		return nil, fmt.Errorf("database error (query 2): %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			return nil, fmt.Errorf("database scan error (query 2): %v", err)
		}
		contacts[username] = struct{}{}
	}

	// Convert map keys to slice
	contactList := make([]string, 0, len(contacts))
	for contact := range contacts {
		contactList = append(contactList, contact)
	}
	return contactList, nil
}

// ---- Message Methods ----

// SendMessage inserts a new encrypted message.
func (s *PostgresStore) SendMessage(ctx context.Context, senderID int, recipientUsername, senderBlob, recipientBlob string) error {
	recipientID, err := s.GetUserIDByUsername(ctx, recipientUsername)
	if err != nil {
		return fmt.Errorf("recipient user not found")
	}

	_, err = s.db.Exec(ctx,
		"INSERT INTO messages (sender_id, recipient_id, sender_blob, recipient_blob) VALUES ($1, $2, $3, $4)",
		senderID, recipientID, senderBlob, recipientBlob,
	)
	if err != nil {
		return fmt.Errorf("database error: %v", err)
	}
	return nil
}

// Message struct for get_messages response
type Message struct {
	ID             int       `json:"id"`
	SenderID       int       `json:"sender_id"`
	RecipientID    int       `json:"recipient_id"`
	Timestamp      time.Time `json:"timestamp"`
	SenderUsername string    `json:"sender_username"`
	EncryptedBlob  string    `json:"encrypted_blob"`
}

// GetMessages fetches new messages between two users.
func (s *PostgresStore) GetMessages(ctx context.Context, myID int, partnerUsername string, sinceID int) ([]Message, error) {
	partnerID, err := s.GetUserIDByUsername(ctx, partnerUsername)
	if err != nil {
		return nil, fmt.Errorf("partner user not found")
	}

	rows, err := s.db.Query(ctx,
		`
        SELECT 
            m.id, 
            m.sender_id, 
            m.recipient_id, 
            m.timestamp, 
            u_sender.username AS sender_username,
            CASE
                WHEN m.sender_id = $1 THEN m.sender_blob
                ELSE m.recipient_blob
            END AS encrypted_blob
        FROM messages m
        JOIN users u_sender ON u_sender.id = m.sender_id
        WHERE 
            ((m.sender_id = $1 AND m.recipient_id = $2) OR (m.sender_id = $2 AND m.recipient_id = $1))
            AND m.id > $3
        ORDER BY m.timestamp ASC
        `,
		myID, partnerID, sinceID)

	if err != nil {
		return nil, fmt.Errorf("database error: %v", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		if err := rows.Scan(&msg.ID, &msg.SenderID, &msg.RecipientID, &msg.Timestamp, &msg.SenderUsername, &msg.EncryptedBlob); err != nil {
			return nil, fmt.Errorf("database scan error: %v", err)
		}
		messages = append(messages, msg)
	}
	return messages, nil
}
