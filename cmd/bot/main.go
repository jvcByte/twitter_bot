package main

import (
	"log"

	"github.com/jvcByte/twitter_bot/internal/bot"
	"github.com/jvcByte/twitter_bot/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	if cfg.TwitterUsername == "" {
		log.Fatal("TWITTER_USERNAME must be set")
	}
	bot.Run(cfg)
}
