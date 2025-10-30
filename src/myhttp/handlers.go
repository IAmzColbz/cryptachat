package myhttp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cryptachat-server/store" // Import store

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// A helper function to write JSON errors
func (s *Server) writeJSONError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"message": message})
}

// A helper function to write JSON responses
func (s *Server) writeJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// --- Auth Handlers ---

// Define the expected JSON payload for registration/login
type authPayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// handleRegister returns the handler function for the /register route
func (s *Server) handleRegister() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload authPayload

		// 1. Parse the JSON body
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			s.writeJSONError(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		// 2. Validate input
		if payload.Username == "" || payload.Password == "" {
			s.writeJSONError(w, "Missing username or password", http.StatusBadRequest)
			return
		}

		// 3. Hash the password (using bcrypt)
		hash, err := bcrypt.GenerateFromPassword([]byte(payload.Password), bcrypt.DefaultCost)
		if err != nil {
			s.writeJSONError(w, fmt.Sprintf("Failed to hash password: %v", err), http.StatusInternalServerError)
			return
		}

		// 4. Call the database logic
		err = s.store.RegisterUser(r.Context(), payload.Username, string(hash))
		if err != nil {
			if err.Error() == "username already exists" {
				s.writeJSONError(w, "Username already exists.", http.StatusConflict) // 409
			} else {
				s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		// 5. Send success response
		s.writeJSON(w, map[string]string{"message": "New user registered successfully!"}, http.StatusCreated)
	}
}

// handleLogin returns the handler for the /login route
func (s *Server) handleLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload authPayload

		// 1. Parse the JSON body
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			s.writeJSONError(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		// 2. Validate input
		if payload.Username == "" || payload.Password == "" {
			s.writeJSONError(w, "Could not verify", http.StatusUnauthorized) // 401
			return
		}

		// 3. Get user from DB
		user, err := s.store.GetUserByUsername(r.Context(), payload.Username)
		if err != nil {
			s.writeJSONError(w, "Could not verify! Check username/password.", http.StatusUnauthorized)
			return
		}

		// 4. Check password
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(payload.Password)); err != nil {
			s.writeJSONError(w, "Could not verify! Check username/password.", http.StatusUnauthorized)
			return
		}

		// 5. Create JWT token
		// Define your claims struct (must match what auth middleware expects)
		type AppClaims struct {
			UserID   int    `json:"user_id"`
			Username string `json:"username"`
			jwt.RegisteredClaims
		}

		claims := AppClaims{
			UserID:   user.ID,
			Username: user.Username,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(s.cfg.JWTSecret))
		if err != nil {
			s.writeJSONError(w, fmt.Sprintf("Error creating token: %v", err), http.StatusInternalServerError)
			return
		}

		// 6. Send token
		s.writeJSON(w, map[string]string{"token": tokenString}, http.StatusOK)
	}
}

// --- Key Handlers ---

type keyPayload struct {
	PublicKey string `json:"public_key"`
}

func (s *Server) handleUploadKey() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, ok := s.getUserFromContext(r)
		if !ok {
			s.writeJSONError(w, "Could not get user from context", http.StatusInternalServerError)
			return
		}

		var payload keyPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			s.writeJSONError(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if payload.PublicKey == "" {
			s.writeJSONError(w, "Missing public_key", http.StatusBadRequest)
			return
		}

		if err := s.store.UploadPublicKey(r.Context(), currentUser.ID, payload.PublicKey); err != nil {
			s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		s.writeJSON(w, map[string]string{"message": "Public key uploaded successfully."}, http.StatusOK)
	}
}

func (s *Server) handleGetKey() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		usernameToFind := r.URL.Query().Get("username")
		if usernameToFind == "" {
			s.writeJSONError(w, "Missing username query parameter.", http.StatusBadRequest)
			return
		}

		key, err := s.store.GetPublicKeyByUsername(r.Context(), usernameToFind)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				s.writeJSONError(w, "User not found or has no public key.", http.StatusNotFound)
			} else {
				s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		s.writeJSON(w, map[string]string{
			"username":   usernameToFind,
			"public_key": key,
		}, http.StatusOK)
	}
}

// --- Chat Request Handlers ---

type chatRequestPayload struct {
	RecipientUsername string `json:"recipient_username"`
	RequesterUsername string `json:"requester_username"`
}

func (s *Server) handleRequestChat() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, ok := s.getUserFromContext(r)
		if !ok {
			s.writeJSONError(w, "Could not get user from context", http.StatusInternalServerError)
			return
		}

		var payload chatRequestPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			s.writeJSONError(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if payload.RecipientUsername == "" {
			s.writeJSONError(w, "Missing recipient_username", http.StatusBadRequest)
			return
		}

		err := s.store.RequestChat(r.Context(), currentUser.ID, payload.RecipientUsername)
		if err != nil {
			if strings.Contains(err.Error(), "recipient user not found") {
				s.writeJSONError(w, "Recipient user not found.", http.StatusNotFound)
			} else if strings.Contains(err.Error(), "already pending") {
				s.writeJSONError(w, "Chat request already pending or accepted.", http.StatusConflict)
			} else if strings.Contains(err.Error(), "yourself") {
				s.writeJSONError(w, "Cannot send chat request to yourself.", http.StatusBadRequest)
			} else {
				s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		s.writeJSON(w, map[string]string{"message": fmt.Sprintf("Chat request sent to %s.", payload.RecipientUsername)}, http.StatusCreated)
	}
}

func (s *Server) handleGetChatRequests() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, ok := s.getUserFromContext(r)
		if !ok {
			s.writeJSONError(w, "Could not get user from context", http.StatusInternalServerError)
			return
		}

		requests, err := s.store.GetChatRequests(r.Context(), currentUser.ID)
		if err != nil {
			s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		s.writeJSON(w, map[string][]store.PendingRequest{"pending_requests": requests}, http.StatusOK)
	}
}

func (s *Server) handleAcceptChat() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, ok := s.getUserFromContext(r)
		if !ok {
			s.writeJSONError(w, "Could not get user from context", http.StatusInternalServerError)
			return
		}

		var payload chatRequestPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			s.writeJSONError(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if payload.RequesterUsername == "" {
			s.writeJSONError(w, "Missing requester_username", http.StatusBadRequest)
			return
		}

		err := s.store.AcceptChat(r.Context(), currentUser.ID, payload.RequesterUsername)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				s.writeJSONError(w, "No pending request found from that user.", http.StatusNotFound)
			} else {
				s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		s.writeJSON(w, map[string]string{"message": fmt.Sprintf("Chat request from %s accepted!", payload.RequesterUsername)}, http.StatusOK)
	}
}

func (s *Server) handleGetContacts() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, ok := s.getUserFromContext(r)
		if !ok {
			s.writeJSONError(w, "Could not get user from context", http.StatusInternalServerError)
			return
		}

		contacts, err := s.store.GetContacts(r.Context(), currentUser.ID)
		if err != nil {
			s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		s.writeJSON(w, map[string][]string{"contacts": contacts}, http.StatusOK)
	}
}

// --- Message Handlers ---

type sendMessagePayload struct {
	RecipientUsername string `json:"recipient_username"`
	SenderBlob        string `json:"sender_blob"`
	RecipientBlob     string `json:"recipient_blob"`
}

func (s *Server) handleSendMessage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, ok := s.getUserFromContext(r)
		if !ok {
			s.writeJSONError(w, "Could not get user from context", http.StatusInternalServerError)
			return
		}

		var payload sendMessagePayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			s.writeJSONError(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if payload.RecipientUsername == "" || payload.SenderBlob == "" || payload.RecipientBlob == "" {
			s.writeJSONError(w, "Missing recipient_username, sender_blob, or recipient_blob", http.StatusBadRequest)
			return
		}

		err := s.store.SendMessage(r.Context(), currentUser.ID, payload.RecipientUsername, payload.SenderBlob, payload.RecipientBlob)
		if err != nil {
			if strings.Contains(err.Error(), "recipient user not found") {
				s.writeJSONError(w, "Recipient user not found.", http.StatusNotFound)
			} else {
				s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		s.writeJSON(w, map[string]string{"message": "Message sent successfully."}, http.StatusCreated)
	}
}

func (s *Server) handleGetMessages() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, ok := s.getUserFromContext(r)
		if !ok {
			s.writeJSONError(w, "Could not get user from context", http.StatusInternalServerError)
			return
		}

		partnerUsername := r.URL.Query().Get("username")
		if partnerUsername == "" {
			s.writeJSONError(w, "Missing username query parameter.", http.StatusBadRequest)
			return
		}

		sinceIDStr := r.URL.Query().Get("since_id")
		if sinceIDStr == "" {
			sinceIDStr = "0"
		}
		sinceID, err := strconv.Atoi(sinceIDStr)
		if err != nil {
			s.writeJSONError(w, "Invalid since_id parameter, must be an integer.", http.StatusBadRequest)
			return
		}

		messages, err := s.store.GetMessages(r.Context(), currentUser.ID, partnerUsername, sinceID)
		if err != nil {
			if strings.Contains(err.Error(), "partner user not found") {
				s.writeJSONError(w, "Partner user not found.", http.StatusNotFound)
			} else {
				s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		s.writeJSON(w, map[string][]store.Message{"messages": messages}, http.StatusOK)
	}
}
