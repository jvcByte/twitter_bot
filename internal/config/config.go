package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	TwitterUsername string
	TwitterPassword string

	// FeedsFile is the path to the JSON feeds file.
	FeedsFile string

	// Category filters by category field in the feeds file. Empty = all.
	Category string

	// PollInterval is how often to poll feeds in continuous mode.
	PollInterval time.Duration

	// MaxArticleAge ignores articles older than this.
	MaxArticleAge time.Duration

	// TweetDelay is the minimum gap between consecutive tweets.
	TweetDelay time.Duration

	// MaxTweetsPerRun caps tweets per run. 0 = unlimited.
	MaxTweetsPerRun int

	// PostMode controls content type: news | meme | mixed | creator | engage
	PostMode string

	// GroqAPIKey is the Groq API key for LLM calls.
	GroqAPIKey string

	// ImgflipUsername and ImgflipPassword are optional meme image credentials.
	ImgflipUsername string
	ImgflipPassword string
}

// Load reads .env and environment variables and returns a populated Config.
func Load() (*Config, error) {
	godotenv.Load() //nolint — optional, no .env in CI

	return &Config{
		TwitterUsername: os.Getenv("TWITTER_USERNAME"),
		TwitterPassword: os.Getenv("TWITTER_PASSWORD"),
		FeedsFile:       envString("FEEDS_FILE", "data/rss_feeds.json"),
		Category:        os.Getenv("CATEGORY"),
		PollInterval:    envDuration("POLL_INTERVAL_MINUTES", 5) * time.Minute,
		MaxArticleAge:   envDuration("MAX_ARTICLE_AGE_HOURS", 2) * time.Hour,
		TweetDelay:      envDuration("TWEET_DELAY_SECONDS", 90) * time.Second,
		MaxTweetsPerRun: int(envDuration("MAX_TWEETS_PER_RUN", 5)),
		PostMode:        envString("POST_MODE", "news"),
		GroqAPIKey:      os.Getenv("GROQ_API_KEY"),
		ImgflipUsername: os.Getenv("IMGFLIP_USERNAME"),
		ImgflipPassword: os.Getenv("IMGFLIP_PASSWORD"),
	}, nil
}

func envString(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func envDuration(key string, defaultVal int64) time.Duration {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return time.Duration(n)
		}
	}
	return time.Duration(defaultVal)
}
