package config

import (
	"os"
	"github.com/joho/godotenv"
)

type Config struct {
	TwitterAPIKey       string
	TwitterAPISecret    string
	TwitterAccessToken  string
	TwitterAccessSecret string
	HuggingFaceAPIKey   string
	PostMode            string
}

func Load() (*Config, error) {
	godotenv.Load()

	return &Config{
		TwitterAPIKey:       os.Getenv("TWITTER_API_KEY"),
		TwitterAPISecret:    os.Getenv("TWITTER_API_SECRET"),
		TwitterAccessToken:  os.Getenv("TWITTER_ACCESS_TOKEN"),
		TwitterAccessSecret: os.Getenv("TWITTER_ACCESS_SECRET"),
		HuggingFaceAPIKey:   os.Getenv("HUGGINGFACE_API_KEY"),
		PostMode:            getEnvOrDefault("POST_MODE", "random"),
	}, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
