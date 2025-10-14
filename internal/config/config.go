package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
	JWTSecret   string
	ServerPort  string
}

func Load() *Config {
	// Load .env file if it exists (for local development)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on system environment variables")
	}

	cfg := &Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		ServerPort:  os.Getenv("SERVER_PORT"),
	}

	if cfg.ServerPort == "" {
		cfg.ServerPort = ":8080"
	}

	// Validate required fields
	if cfg.DatabaseURL == "" {
		log.Fatal("Missing required environment variable: DATABASE_URL")
	}
	if cfg.JWTSecret == "" {
		log.Fatal("Missing required environment variable: JWT_SECRET")
	}

	return cfg
}
