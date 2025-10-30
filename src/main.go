package main

import (
	"log"
	"net/http"
	"os"

	"cryptachat-server/config"
	"cryptachat-server/myhttp" // Your new http package
	"cryptachat-server/store"
)

func main() {
	// Load config. This will load from .config/docker.env for local dev
	// or from environment variables (injected by Docker Compose) in production.
	cfg, err := config.LoadConfig("../.config/docker.env")
	if err != nil {
		log.Printf("Warning: could not load .env file. Will rely on environment variables. Error: %v", err)
		// Try again, this time relying *only* on environment variables
		cfg, err = config.LoadConfig("")
		if err != nil {
			log.Fatalf("FATAL: could not load configuration from environment: %v", err)
		}
	}

	// *** FIX: Changed path from "../server/schema.sql" to "./server/schema.sql" ***
	// This path is now correct relative to the binary's location in the /app container directory
	dbStore, err := store.NewPostgresStore(cfg.DatabaseURL, "./server/schema.sql")
	if err != nil {
		log.Fatalf("FATAL: could not connect to database: %v", err)
	}
	defer dbStore.Close() // Make sure to close the DB connection on exit
	log.Println("Database connection established and schema initialized.")

	// Init http
	server := myhttp.NewServer(cfg, dbStore)
	log.Println("HTTP server initialized.")

	// Get port from environment, default to 5000
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
