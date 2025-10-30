// src/myhttp/handlers_ws.go
package myhttp

import (
	"cryptachat-server/websockets"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// We need to check the origin to prevent CSRF attacks
	CheckOrigin: func(r *http.Request) bool {
		// TODO: For production, you should validate the origin.
		// Example:
		// origin := r.Header.Get("Origin")
		// return origin == "http://your-frontend-domain.com"
		return true // Allow all for now
	},
}

// handleServeWS upgrades the connection and registers the client
func (s *Server) handleServeWS() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Get user from context (set by jwtAuthMiddleware)
		currentUser, ok := s.getUserFromContext(r)
		if !ok {
			s.writeJSONError(w, "Could not get user from context", http.StatusInternalServerError)
			return
		}

		// 2. Upgrade connection
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WS: Failed to upgrade connection for user %d: %v", currentUser.ID, err)
			return
		}

		// 3. Create and register the client
		client := websockets.NewClient(s.hub, conn, currentUser.ID)
		client.Register() // This will send the client to the hub's register channel

		// 4. Start the client's read/write pumps in separate goroutines
		go client.WritePump()
		go client.ReadPump()
	}
}
