package http

import (
	"encoding/json"
	"fmt"
	"net/http"

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

// Define the expected JSON payload for registration
type registerPayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// handleRegister returns the handler function for the /register route
func (s *Server) handleRegister() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload registerPayload

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
