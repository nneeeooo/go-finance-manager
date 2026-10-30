package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	AppHost string
	AppPort string
}

func Load() *Config {
	if err := godotenv.Load("../.env"); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	return &Config{
		AppHost: os.Getenv("APP_HOST"),
		AppPort: os.Getenv("APP_PORT"),
	}
}
