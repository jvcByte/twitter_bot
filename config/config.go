package config

import (
	"os"
	"github.com/joho/godotenv"
)

type Config struct {
	TwitterUsername     string
	TwitterPassword     string
	HuggingFaceAPIKey   string
	PostMode            string
}

func Load() (*Config, error) {
	godotenv.Load()

	return &Config{
		TwitterUsername:   os.Getenv("TWITTER_USERNAME"),
		TwitterPassword:   os.Getenv("TWITTER_PASSWORD"),
		HuggingFaceAPIKey: os.Getenv("HUGGINGFACE_API_KEY"),
		PostMode:          getEnvOrDefault("POST_MODE", "random"),
	}, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
