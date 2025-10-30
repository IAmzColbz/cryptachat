package main

import (
	"log"
	"net/http" // Import net/http
	"os"

	"cryptachat-server/config"
	"cryptachat-server/http" // Your new http package
	"cryptachat-server/store"
)

func main() {
	// ... (LoadConfig and NewPostgresStore are the same)
	cfg, err := config.LoadConfig("../.config/docker.env")
	if err != nil {
		log.Fatalf("FATAL: could not load configuration file: %v", err)
	}

	dbStore, err := store.NewPostgresStore(cfg.DatabaseURL, "../server/schema.sql")
	if err != nil {
		log.Fatalf("FATAL: could not connect to database: %v", err)
	}
	defer dbStore.Close() // Make sure to close the DB connection on exit
	log.Println("Database connection established and schema initialized.")

	// Init http
	server := http.NewServer(cfg, dbStore)
	log.Println("HTTP server initialized.")

	// ... (Get port is the same)
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
