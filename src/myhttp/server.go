package myhttp

import (
	"cryptachat-server/config"
	"cryptachat-server/store" // Your store package
	"net/http"
)

// Server holds the dependencies for your HTTP handlers.
type Server struct {
	store *store.PostgresStore
	cfg   *config.Config
	mux   *http.ServeMux // The HTTP router
}

// NewServer creates a new server instance.
func NewServer(cfg *config.Config, store *store.PostgresStore) *Server {
	s := &Server{
		store: store,
		cfg:   cfg,
		mux:   http.NewServeMux(),
	}
	s.registerRoutes() // Call the method to register all routes
	return s
}

// ServeHTTP makes our Server usable as an http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// TODO: Add logging middleware here
	s.mux.ServeHTTP(w, r)
}

// registerRoutes is the Go equivalent of all your @app.route decorators.
// TODO: Add rate limiting, similar to the Python server's 'flask-limiter'.
// This can be done by wrapping handlers with a rate-limiting middleware.
func (s *Server) registerRoutes() {
	// Auth routes
	s.mux.HandleFunc("POST /register", s.handleRegister())
	s.mux.HandleFunc("POST /login", s.handleLogin())

	// Key routes (Protected)
	s.mux.HandleFunc("POST /upload_key", s.jwtAuthMiddleware(s.handleUploadKey()))
	s.mux.HandleFunc("GET /get_key", s.jwtAuthMiddleware(s.handleGetKey()))

	// Chat/Contact routes (Protected)
	s.mux.HandleFunc("POST /request_chat", s.jwtAuthMiddleware(s.handleRequestChat()))
	s.mux.HandleFunc("GET /get_chat_requests", s.jwtAuthMiddleware(s.handleGetChatRequests()))
	s.mux.HandleFunc("POST /accept_chat", s.jwtAuthMiddleware(s.handleAcceptChat()))
	s.mux.HandleFunc("GET /get_contacts", s.jwtAuthMiddleware(s.handleGetContacts()))

	// Message routes (Protected)
	s.mux.HandleFunc("POST /send_message", s.jwtAuthMiddleware(s.handleSendMessage()))
	s.mux.HandleFunc("GET /get_messages", s.jwtAuthMiddleware(s.handleGetMessages()))
}
