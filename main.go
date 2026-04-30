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

		// Post headline + image as the main tweet (no link = better reach)
		headline := content.FormatHeadline(a)
		fmt.Printf("→ [%s] %s\n", a.FeedName, a.Title)

		imgPath, err := content.DownloadImage(a.ImageURL)
		if err != nil {
			log.Printf("  image download failed: %v — posting text only", err)
		}

		var (
			tweetURL string
			tweetErr error
		)
		if imgPath != "" {
			tweetURL, tweetErr = client.TweetWithMedia(headline, imgPath)
			os.Remove(imgPath)
		} else {
			tweetURL, tweetErr = client.Tweet(headline)
		}

		if tweetErr != nil {
			log.Printf("tweet failed: %v", tweetErr)
			continue
		}

		seen.Add(a.Link)
		fmt.Println("  ✓ tweeted")
		tweeted++

		// Reply with the link — keeps it off the main tweet for reach,
		// but still accessible and seeds the reply chain for algorithm boost.
		if tweetURL != "" {
			time.Sleep(3 * time.Second)
			replyText := fmt.Sprintf("🔗 Full story: %s", a.Link)
			if err := client.ReplyTo(tweetURL, replyText); err != nil {
				log.Printf("  link reply failed: %v", err)
			} else {
				fmt.Println("  ✓ link reply posted")
			}
		}

		time.Sleep(cfg.TweetDelay)
	}
}

func runMeme(client *twitter.Client, seen *content.SeenStore, cfg *config.Config, headline string) {
	if cfg.GroqAPIKey == "" {
		log.Printf("GROQ_API_KEY not set — skipping meme post")
		return
	}

	// ~30% chance to post a full thread instead of a single tweet
	if rand.Intn(10) < 3 {
		runThread(client, cfg, headline)
		return
	}

	post, err := content.GenerateMemePost(cfg.GroqAPIKey, headline)
	if err != nil {
		log.Printf("meme generation failed: %v", err)
		return
	}

	fmt.Printf("→ [AI meme] %s\n", post)

	top, bottom := splitMemeText(post)
	imgPath, err := content.GenerateMemeImage(cfg.ImgflipUsername, cfg.ImgflipPassword, top, bottom)
	if err != nil {
		log.Printf("  meme image failed: %v — posting text only", err)
	}

	var tweetErr error
	if imgPath != "" {
		_, tweetErr = client.TweetWithMedia(post, imgPath)
		os.Remove(imgPath)
	} else {
		_, tweetErr = client.Tweet(post)
	}

	if tweetErr != nil {
		log.Printf("tweet failed: %v", tweetErr)
		return
	}
	fmt.Println("  ✓ tweeted")
}

// runThread generates and posts a multi-tweet thread via Groq.
func runThread(client *twitter.Client, cfg *config.Config, topic string) {
	tweets, err := content.GenerateThread(cfg.GroqAPIKey, topic)
	if err != nil {
		log.Printf("thread generation failed: %v — falling back to single post", err)
		// Fall back to single meme post
		post, err := content.GenerateMemePost(cfg.GroqAPIKey, topic)
		if err != nil {
			log.Printf("fallback meme failed: %v", err)
			return
		}
		if _, err := client.Tweet(post); err != nil {
			log.Printf("tweet failed: %v", err)
		}
		return
	}

	fmt.Printf("→ [AI thread] %d tweets | %s\n", len(tweets), tweets[0])

	if _, err := client.Thread(tweets, ""); err != nil {
		log.Printf("thread post failed: %v", err)
		return
	}
	fmt.Println("  ✓ thread posted")
}

// splitMemeText splits a post into top/bottom text for meme templates
func splitMemeText(post string) (string, string) {
	lines := strings.SplitN(post, "\n", 2)
	if len(lines) == 2 {
		return strings.TrimSpace(lines[0]), strings.TrimSpace(lines[1])
	}
	words := strings.Fields(post)
	if len(words) <= 2 {
		return post, ""
	}
	mid := len(words) / 2
	return strings.Join(words[:mid], " "), strings.Join(words[mid:], " ")
}

func runMixed(client *twitter.Client, seen *content.SeenStore, cfg *config.Config) {
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
			runMeme(client, seen, cfg, a.Title)
			memeInserted = true
			time.Sleep(cfg.TweetDelay)
			continue
		}

		// Post headline without link for reach; reply with link
		headline := content.FormatHeadline(a)
		fmt.Printf("→ [%s] %s\n", a.FeedName, a.Title)

		tweetURL, err := client.Tweet(headline)
		if err != nil {
			log.Printf("tweet failed: %v", err)
			continue
		}

		seen.Add(a.Link)
		fmt.Println("  ✓ tweeted")
		tweeted++

		if tweetURL != "" {
			time.Sleep(3 * time.Second)
			replyText := fmt.Sprintf("🔗 Full story: %s", a.Link)
			if err := client.ReplyTo(tweetURL, replyText); err != nil {
				log.Printf("  link reply failed: %v", err)
			} else {
				fmt.Println("  ✓ link reply posted")
			}
		}

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
