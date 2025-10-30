package http

import (
	"context"
	"cryptachat-server/store" // Import the store package
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// A custom context key to pass user info
type contextKey string

const userContextKey = contextKey("user")

// jwtAuthMiddleware is the Go equivalent of your @token_required decorator
func (s *Server) jwtAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			s.writeJSONError(w, "Token is missing!", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			s.writeJSONError(w, "Invalid token format", http.StatusUnauthorized)
			return
		}

		// Define your claims struct (must match what you create at login)
		type AppClaims struct {
			UserID   int    `json:"user_id"`
			Username string `json:"username"`
			jwt.RegisteredClaims
		}

		token, err := jwt.ParseWithClaims(tokenString, &AppClaims{}, func(token *jwt.Token) (interface{}, error) {
			// Validate the signing method
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			// Return the secret key (from your config)
			return []byte(s.cfg.JWTSecret), nil
		})

		if err != nil {
			if err == jwt.ErrTokenExpired {
				s.writeJSONError(w, "Token has expired!", http.StatusUnauthorized)
			} else {
				s.writeJSONError(w, fmt.Sprintf("Token is invalid: %v", err), http.StatusUnauthorized)
			}
			return
		}

		if claims, ok := token.Claims.(*AppClaims); ok && token.Valid {
			// In your Python code, you double-check the user against the DB.
			// This is critical, and we do it here.
			user, err := s.store.GetUserByID(r.Context(), claims.UserID)
			if err != nil || user == nil {
				s.writeJSONError(w, "Token is invalid!", http.StatusUnauthorized)
				return
			}

			// This is the Go way to pass "current_user" to the next handler
			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))

		} else {
			s.writeJSONError(w, "Token is invalid!", http.StatusUnauthorized)
		}
	}
}

// getUserFromContext is a helper to retrieve the user from the context.
func (s *Server) getUserFromContext(r *http.Request) (*store.User, bool) {
	user, ok := r.Context().Value(userContextKey).(*store.User)
	return user, ok
}
