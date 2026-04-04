package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
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

	fmt.Printf("🚀 News bot started | mode: %s | feeds: %d | category: %s\n",
		cfg.PostMode, len(feeds), categoryLabel)

	runOnce := os.Getenv("RUN_ONCE") == "true"

	if runOnce {
		runPoll(client, seen, cfg)
	} else {
		fmt.Printf("⏱  poll every %v | max age %v | tweet delay %v\n\n",
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

	switch cfg.PostMode {
	case "meme":
		runMeme(client, seen, cfg, "")
	case "mixed":
		runMixed(client, seen, cfg)
	default: // "news"
		runNews(client, seen, cfg)
	}
}

func runNews(client *twitter.Client, seen *content.SeenStore, cfg *config.Config) {
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

		// Try to attach article image
		imgPath, err := content.DownloadImage(a.ImageURL)
		if err != nil {
			log.Printf("  image download failed: %v — posting text only", err)
		}

		var tweetErr error
		if imgPath != "" {
			tweetErr = client.TweetWithMedia(post, imgPath)
			os.Remove(imgPath)
		} else {
			tweetErr = client.Tweet(post)
		}

		if tweetErr != nil {
			log.Printf("tweet failed: %v", tweetErr)
			continue
		}

		seen.Add(a.Link)
		fmt.Println("  ✓ tweeted")
		tweeted++
		time.Sleep(cfg.TweetDelay)
	}
}

func runMeme(client *twitter.Client, seen *content.SeenStore, cfg *config.Config, headline string) {
	if cfg.GroqAPIKey == "" {
		log.Printf("GROQ_API_KEY not set — skipping meme post")
		return
	}

	post, err := content.GenerateMemePost(cfg.GroqAPIKey, headline)
	if err != nil {
		log.Printf("meme generation failed: %v", err)
		return
	}

	fmt.Printf("→ [AI meme] %s\n", post)

	// Try to generate a meme image via Imgflip
	// Split post roughly in half for top/bottom text
	top, bottom := splitMemeText(post)
	imgPath, err := content.GenerateMemeImage(cfg.ImgflipUsername, cfg.ImgflipPassword, top, bottom)
	if err != nil {
		log.Printf("  meme image failed: %v — posting text only", err)
	}

	var tweetErr error
	if imgPath != "" {
		tweetErr = client.TweetWithMedia(post, imgPath)
		os.Remove(imgPath)
	} else {
		tweetErr = client.Tweet(post)
	}

	if tweetErr != nil {
		log.Printf("tweet failed: %v", tweetErr)
		return
	}
	fmt.Println("  ✓ tweeted")
}

// splitMemeText splits a post into top/bottom text for meme templates
func splitMemeText(post string) (string, string) {
	lines := strings.SplitN(post, "\n", 2)
	if len(lines) == 2 {
		return strings.TrimSpace(lines[0]), strings.TrimSpace(lines[1])
	}
	// Split at midpoint word boundary
	words := strings.Fields(post)
	if len(words) <= 2 {
		return post, ""
	}
	mid := len(words) / 2
	return strings.Join(words[:mid], " "), strings.Join(words[mid:], " ")
}

func runMixed(client *twitter.Client, seen *content.SeenStore, cfg *config.Config) {
	// Fetch news articles first — we'll use the top headline for reaction posts
	articles, err := content.Poll(seen, cfg.MaxArticleAge, cfg.FeedsFile, cfg.Category)
	if err != nil {
		log.Printf("poll error: %v", err)
		return
	}

	fmt.Printf("found %d new articles\n", len(articles))

	tweeted := 0
	memeInserted := false

	for _, a := range articles {
		if cfg.MaxTweetsPerRun > 0 && tweeted >= cfg.MaxTweetsPerRun {
			fmt.Printf("reached max %d tweets per run\n", cfg.MaxTweetsPerRun)
			break
		}

		// Insert one meme/humor post roughly in the middle of the run
		if !memeInserted && tweeted == cfg.MaxTweetsPerRun/2 {
			headline := a.Title // use current headline for reaction format
			runMeme(client, seen, cfg, headline)
			memeInserted = true
			time.Sleep(cfg.TweetDelay)
			continue
		}

		post := content.Format(a)
		fmt.Printf("→ [%s] %s\n", a.FeedName, a.Title)

		if err := client.Tweet(post); err != nil {
			log.Printf("tweet failed: %v", err)
			continue
		}

		seen.Add(a.Link)
		fmt.Println("  ✓ tweeted")
		tweeted++

		// Occasionally inject a standalone meme between news posts
		if !memeInserted && tweeted > 0 && rand.Intn(3) == 0 {
			time.Sleep(cfg.TweetDelay)
			runMeme(client, seen, cfg, "")
			memeInserted = true
		}

		time.Sleep(cfg.TweetDelay)
	}

	// If no meme was inserted yet (e.g. no articles), post one standalone
	if !memeInserted && cfg.GroqAPIKey != "" {
		runMeme(client, seen, cfg, "")
	}
}
