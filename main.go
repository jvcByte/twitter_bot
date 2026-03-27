package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/yourusername/twitter-bot/config"
	"github.com/yourusername/twitter-bot/content"
	"github.com/yourusername/twitter-bot/twitter"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	twitterClient := twitter.NewClient(
		cfg.TwitterAPIKey,
		cfg.TwitterAPISecret,
		cfg.TwitterAccessToken,
		cfg.TwitterAccessSecret,
	)

	rand.Seed(time.Now().UnixNano())
	contentType := rand.Intn(3)

	var post string

	switch contentType {
	case 0:
		fmt.Println("📝 Generating template post...")
		post, err = content.GetTemplatePost()
	case 1:
		fmt.Println("📰 Fetching RSS post...")
		post, err = content.GetRSSPost()
	case 2:
		fmt.Println("🤖 Generating AI post...")
		post, err = content.GenerateAIPost(cfg.HuggingFaceAPIKey)
	}

	if err != nil {
		log.Fatalf("Failed to generate content: %v", err)
	}

	fmt.Printf("\n📤 Posting: %s\n\n", post)

	if err := twitterClient.Tweet(post); err != nil {
		log.Fatalf("Failed to post tweet: %v", err)
	}

	fmt.Println("✨ Done!")
}
