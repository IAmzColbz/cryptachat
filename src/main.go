package main

import (
	"log"
	"os"

	"cryptachat-server/config"
	"cryptachat-server/http"
	"cryptachat-server/store"
)

func main() {
	// Load config
	cfg, err := config.LoadConfig("../.config/docker.env")
	if err != nil {
		log.Fatalf("FATAL: could not load configuration file: %v", err)
	}

	// Init DB Store
	dbStore, err := store.NewPostgresStore(cfg.DatabaseURL, "../server/schema.sql")
	if err != nil {
		log.Fatalf("FATAL: could not connect to database: %v", err)
	}
	log.Println("Database connection establishand and schema initialized.")

	// Init http
	server := http.NewServer(cfg, dbStore)
	log.Println("HTTP server initialized.")

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}
}
