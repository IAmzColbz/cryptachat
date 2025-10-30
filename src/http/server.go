package http

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
	s.mux.ServeHTTP(w, r)
}

// registerRoutes is the Go equivalent of all your @app.route decorators.
func (s *Server) registerRoutes() {
	s.mux.HandleFunc("POST /register", s.handleRegister())
	// TODO: Add other routes
	// s.mux.HandleFunc("POST /login", s.handleLogin())
	// s.mux.HandleFunc("POST /upload_key", s.handleUploadKey())
	// ...
}
