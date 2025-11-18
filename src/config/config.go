package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
	JWTSecret   string

	dbHost     string
	dbPort     string
	dbUser     string
	dbPassword string
	dbName     string
}

func LoadConfig(path string) (*Config, error) {
	_ = godotenv.Load(path)

	cfg := &Config{
		dbHost:     os.Getenv("DB_HOST"),
		dbPort:     os.Getenv("DB_PORT"),
		dbUser:     os.Getenv("POSTGRES_USER"),
		dbPassword: os.Getenv("POSTGRES_PASSWORD"),
		dbName:     os.Getenv("POSTGRES_DB"),
		JWTSecret:  os.Getenv("SECRET_KEY"),
	}

	if cfg.dbHost == "" || cfg.dbPort == "" || cfg.dbUser == "" || cfg.dbName == "" {
		return nil, fmt.Errorf("err: one or more database env variables are missing")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("err: SECRET_KEY env variable is missing")
	}

	cfg.DatabaseURL = fmt.Sprintf("postgresql://%s:%s@%s:%s/%s",
		cfg.dbUser, cfg.dbPassword, cfg.dbHost, cfg.dbPort, cfg.dbName,
	)

	return cfg, nil
}
