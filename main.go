package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jvcByte/twitter_bot/config"
	"github.com/jvcByte/twitter_bot/content"
	"github.com/jvcByte/twitter_bot/twitter"
)

const seenStorePath = "data/seen_articles.json"

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if cfg.TwitterUsername == "" || cfg.TwitterPassword == "" {
		log.Fatal("TWITTER_USERNAME and TWITTER_PASSWORD must be set")
	}

	client := twitter.NewClient(cfg.TwitterUsername, cfg.TwitterPassword, "", "")
	seen := content.NewSeenStore(seenStorePath)

	feeds, err := content.LoadFeeds(cfg.FeedsFile)
	if err != nil {
		log.Fatalf("failed to load feeds: %v", err)
	}

	categoryLabel := cfg.Category
	if categoryLabel == "" {
		categoryLabel = "all"
	}

	fmt.Printf("News bot started\n")
	fmt.Printf("%d feeds loaded | category: %s\n", len(feeds), categoryLabel)

	// RUN_ONCE=true → single poll then exit (for GitHub Actions / cron)
	// default       → continuous loop (for self-hosted / Docker)
	runOnce := os.Getenv("RUN_ONCE") == "true"

	if runOnce {
		runPoll(client, seen, cfg)
	} else {
		fmt.Printf("poll every %v | max age %v | tweet delay %v\n\n",
			cfg.PollInterval, cfg.MaxArticleAge, cfg.TweetDelay)
		for {
			runPoll(client, seen, cfg)
			fmt.Printf("sleeping %v...\n\n", cfg.PollInterval)
			time.Sleep(cfg.PollInterval)
		}
	}
}

func runPoll(client *twitter.Client, seen *content.SeenStore, cfg *config.Config) {
	fmt.Printf("[%s] fetching...\n", time.Now().Format("15:04:05"))

	articles, err := content.Poll(seen, cfg.MaxArticleAge, cfg.FeedsFile, cfg.Category)
	if err != nil {
		log.Printf("poll error: %v", err)
		return
	}

	fmt.Printf("found %d new articles\n", len(articles))

	tweeted := 0
	for _, a := range articles {
		if cfg.MaxTweetsPerRun > 0 && tweeted >= cfg.MaxTweetsPerRun {
			fmt.Printf("reached max %d tweets per run\n", cfg.MaxTweetsPerRun)
			break
		}

		post := content.Format(a)
		fmt.Printf("→ [%s] %s\n", a.FeedName, a.Title)

		if err := client.Tweet(post); err != nil {
			log.Printf("tweet failed: %v", err)
			continue
		}

		seen.Add(a.Link)
		fmt.Println("tweeted")
		tweeted++

		time.Sleep(cfg.TweetDelay)
	}
}
