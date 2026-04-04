package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	TwitterUsername string
	TwitterPassword string

	// Path to the feeds JSON file (default: data/rss_feeds.json)
	FeedsFile string

	// Category filter — empty means all categories
	// e.g. "tech", "cybersecurity", "world" — matches category field in rss_feeds.json
	Category string

	// How often to poll all feeds
	PollInterval time.Duration

	// Ignore articles older than this
	MaxArticleAge time.Duration

	// Minimum gap between tweets to avoid rate limits
	TweetDelay time.Duration

	// Max tweets per run (0 = unlimited)
	MaxTweetsPerRun int

	// POST_MODE controls content type: "news", "meme", or "mixed"
	PostMode string

	// Groq API key for AI-generated meme/humor posts
	GroqAPIKey string
}

func Load() (*Config, error) {
	godotenv.Load()

	return &Config{
		TwitterUsername: os.Getenv("TWITTER_USERNAME"),
		TwitterPassword: os.Getenv("TWITTER_PASSWORD"),
		FeedsFile:       envString("FEEDS_FILE", "data/rss_feeds.json"),
		Category:        os.Getenv("CATEGORY"), // optional
		PollInterval:    envDuration("POLL_INTERVAL_MINUTES", 5) * time.Minute,
		MaxArticleAge:   envDuration("MAX_ARTICLE_AGE_HOURS", 2) * time.Hour,
		TweetDelay:      envDuration("TWEET_DELAY_SECONDS", 90) * time.Second,
		MaxTweetsPerRun: int(envDuration("MAX_TWEETS_PER_RUN", 5)),
		PostMode:        envString("POST_MODE", "news"),
		GroqAPIKey:      os.Getenv("GROQ_API_KEY"),
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
