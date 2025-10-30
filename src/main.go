package main

import (
	"log"
	"net/http"
	"os"

	"cryptachat-server/config"
	"cryptachat-server/myhttp" // Your http package
	"cryptachat-server/store"
	"cryptachat-server/websockets" // <-- Import the new websocket package
)

func main() {
	cfg, err := config.LoadConfig("../.config/docker.env")
	if err != nil {
		log.Printf("Warning: could not load .env file. Will rely on environment variables. Error: %v", err)
		cfg, err = config.LoadConfig("")
		if err != nil {
			log.Fatalf("FATAL: could not load configuration from environment: %v", err)
		}
	}

	// ... (database connection logic)
	dbStore, err := store.NewPostgresStore(cfg.DatabaseURL, "./store/schema.sql")
	if err != nil {
		log.Fatalf("FATAL: could not connect to database: %v", err)
	}
	defer dbStore.Close()
	log.Println("Database connection established and schema initialized.")

	// --- WebSocket Hub ---
	// 1. Create the new hub
	hub := websockets.NewHub()
	// 2. Run the hub in its own goroutine
	go hub.Run()
	log.Println("WebSocket hub initialized and running.")
	// ---------------------

	// Init http
	// 3. Pass the hub to the server
	server := myhttp.NewServer(cfg, dbStore, hub)
	log.Println("HTTP server initialized.")

	// ... (port logic)
	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}

	// Start server
	log.Printf("Starting server on :%s", port)
	if err := http.ListenAndServe(":"+port, server); err != nil {
		log.Fatalf("FATAL: could not start server: %v", err)
	}
}
