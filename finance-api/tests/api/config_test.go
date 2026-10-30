package api_test

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	BaseURL string
}

func Load() *Config {
	if err := godotenv.Load("../../../.env"); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	return &Config{
		BaseURL: os.Getenv("TEST_API_BASEURL"),
	}
}
