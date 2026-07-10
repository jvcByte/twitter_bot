package main

import (
	"encoding/json"
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
	case "creator":
		runCreator(client, cfg)
	case "engage":
		runEngagement(client, cfg)
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

		// Post AI-enhanced headline + image (no link = better reach)
		headline := content.FetchAndEngage(a, cfg.GroqAPIKey)
		fmt.Printf("→ [%s] %s\n", a.FeedName, a.Title)
		fmt.Printf("  tweet: %s\n", headline)

		imgPath, err := content.DownloadImage(a.ImageURL)
		if err != nil {
			log.Printf("  image download failed: %v", err)
		}
		// If no article image, generate one with Pollinations
		if imgPath == "" {
			imgPath, err = content.GeneratePollinationsImage(cfg.GroqAPIKey, headline)
			if err != nil {
				log.Printf("  pollinations failed: %v — posting text only", err)
			}
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

		// Self-engage: like, repost, comment — boosts velocity in first few minutes
		if tweetURL != "" {
			time.Sleep(2 * time.Second)
			comment := content.GenerateSelfComment(cfg.GroqAPIKey, headline)
			if err := client.SelfEngage(tweetURL, comment); err != nil {
				log.Printf("  self-engage failed: %v", err)
			}
		}

		// Reply with the link — keeps it off the main tweet for reach,
		// but still accessible and seeds the reply chain for algorithm boost.
		if tweetURL != "" {
			time.Sleep(3 * time.Second)
			replyText := fmt.Sprintf("🔗 Full story: %s", a.Link)
			if _, err := client.ReplyTo(tweetURL, replyText); err != nil {
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

	post, formatName, err := content.GenerateMemePost(cfg.GroqAPIKey, headline)
	if err != nil {
		log.Printf("meme generation failed: %v", err)
		return
	}

	fmt.Printf("→ [AI %s] %s\n", formatName, post)

	// Generate image for all post formats
	var imgPath string
	top, bottom := splitMemeText(post)
	imgPath, err = content.GenerateMemeImageWithGroq(cfg.GroqAPIKey, cfg.ImgflipUsername, cfg.ImgflipPassword, top, bottom)
	if err != nil {
		log.Printf("  image failed: %v — posting text only", err)
	}

	var (
		tweetURL string
		tweetErr error
	)
	if imgPath != "" {
		tweetURL, tweetErr = client.TweetWithMedia(post, imgPath)
		os.Remove(imgPath)
	} else {
		tweetURL, tweetErr = client.Tweet(post)
	}

	if tweetErr != nil {
		log.Printf("tweet failed: %v", tweetErr)
		return
	}
	fmt.Println("  ✓ tweeted")

	// Self-engage immediately after posting
	if tweetURL != "" {
		time.Sleep(2 * time.Second)
		comment := content.GenerateSelfComment(cfg.GroqAPIKey, post)
		if err := client.SelfEngage(tweetURL, comment); err != nil {
			log.Printf("  self-engage failed: %v", err)
		}
	}
}

// runCreator posts owned content — tips, opinions, build logs, PCB insights.
// ~30% chance of a personal thread, otherwise a single post.
func runCreator(client *twitter.Client, cfg *config.Config) {
	if cfg.GroqAPIKey == "" {
		log.Printf("GROQ_API_KEY not set — skipping creator post")
		return
	}

	// ~30% chance to post a thread
	if rand.Intn(10) < 3 {
		tweets, err := content.GenerateCreatorThread(cfg.GroqAPIKey, "")
		if err != nil {
			log.Printf("creator thread failed: %v — falling back to single post", err)
		} else {
			fmt.Printf("→ [creator thread] %d tweets | %s\n", len(tweets), tweets[0])
			threadURL, err := client.Thread(tweets, "")
			if err != nil {
				log.Printf("thread post failed: %v", err)
				return
			}
			fmt.Println("  ✓ thread posted")
			if threadURL != "" {
				time.Sleep(2 * time.Second)
				comment := content.GenerateSelfComment(cfg.GroqAPIKey, tweets[0])
				if err := client.SelfEngage(threadURL, comment); err != nil {
					log.Printf("  self-engage failed: %v", err)
				}
			}
			return
		}
	}

	post, formatName, err := content.GenerateCreatorPost(cfg.GroqAPIKey)
	if err != nil {
		log.Printf("creator post failed: %v", err)
		return
	}

	fmt.Printf("→ [creator %s] %s\n", formatName, post)

	var (
		tweetURL string
		tweetErr error
	)

	// Text-only formats don't benefit from images
	if !content.IsCreatorTextOnly(formatName) {
		top, bottom := splitMemeText(post)
		imgPath, err := content.GenerateMemeImageWithGroq(cfg.GroqAPIKey, cfg.ImgflipUsername, cfg.ImgflipPassword, top, bottom)
		if err != nil {
			log.Printf("  image failed: %v — posting text only", err)
		}
		if imgPath != "" {
			tweetURL, tweetErr = client.TweetWithMedia(post, imgPath)
			os.Remove(imgPath)
		} else {
			tweetURL, tweetErr = client.Tweet(post)
		}
	} else {
		tweetURL, tweetErr = client.Tweet(post)
	}

	if tweetErr != nil {
		log.Printf("tweet failed: %v", tweetErr)
		return
	}
	fmt.Println("  ✓ tweeted")

	if tweetURL != "" {
		time.Sleep(2 * time.Second)
		comment := content.GenerateSelfComment(cfg.GroqAPIKey, post)
		if err := client.SelfEngage(tweetURL, comment); err != nil {
			log.Printf("  self-engage failed: %v", err)
		}
	}
}

// engagementTopics are search queries the bot uses to find relevant posts to engage with.
var engagementTopics = []string{
	"PCB design",
	"embedded systems",
	"firmware development",
	"KiCad",
	"STM32",
	"ESP32",
	"software engineering",
	"clean code",
	"Go programming",
	"Python embedded",
	"#EmbeddedSystems",
	"#PCBDesign",
	"#Firmware",
	"#SoftwareEngineering",
	"#Golang",
	"AI cybersecurity",
	"#DevLife",
}

// runEngagement finds and engages with relevant posts from other users.
func runEngagement(client *twitter.Client, cfg *config.Config) {
	fmt.Println("→ [engage] searching for relevant posts...")

	commentFn := func(tweetText string) string {
		if cfg.GroqAPIKey == "" || len(tweetText) < 20 {
			return ""
		}
		// ~50% chance to comment — not every like needs a comment
		if time.Now().UnixNano()%2 == 0 {
			return ""
		}
		comment, err := content.GenerateEngagementComment(cfg.GroqAPIKey, tweetText)
		if err != nil || comment == "" {
			return ""
		}
		return comment
	}

	n, err := client.EngageWithTopic(engagementTopics, 5, commentFn, 2)
	if err != nil {
		log.Printf("engagement failed: %v", err)
		return
	}
	fmt.Printf("  ✓ engaged with %d posts\n", n)
}

// runThread generates and posts a multi-tweet thread via Groq.
func runThread(client *twitter.Client, cfg *config.Config, topic string) {
	tweets, err := content.GenerateThread(cfg.GroqAPIKey, topic)
	if err != nil {
		log.Printf("thread generation failed: %v — falling back to single post", err)
		// Fall back to single meme post
		post, _, err := content.GenerateMemePost(cfg.GroqAPIKey, topic)
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

	threadURL, err := client.Thread(tweets, "")
	if err != nil {
		log.Printf("thread post failed: %v", err)
		return
	}
	fmt.Println("  ✓ thread posted")

	// Self-engage on the first tweet of the thread
	if threadURL != "" {
		time.Sleep(2 * time.Second)
		comment := content.GenerateSelfComment(cfg.GroqAPIKey, tweets[0])
		if err := client.SelfEngage(threadURL, comment); err != nil {
			log.Printf("  self-engage failed: %v", err)
		}
	}
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
	runRotation(client, seen, cfg)
}

// rotationSlots defines what content type fires on a given slot index.
// Pattern repeats every 8 slots: news, creator, engage, news, meme, creator, engage, news
// At 4 posts/day this gives: 3 news, 2 creator, 1 meme, 2 engage per day.
var rotationSlots = []string{
	"news",
	"creator",
	"engage",
	"news",
	"meme",
	"creator",
	"engage",
	"news",
}

const rotationStatePath = "data/rotation_state.json"

type rotationState struct {
	Slot int `json:"slot"`
}

func loadRotationSlot() int {
	data, err := os.ReadFile(rotationStatePath)
	if err != nil {
		return 0
	}
	var s rotationState
	if err := json.Unmarshal(data, &s); err != nil {
		return 0
	}
	return s.Slot % len(rotationSlots)
}

func saveRotationSlot(slot int) {
	data, _ := json.Marshal(rotationState{Slot: slot})
	os.WriteFile(rotationStatePath, data, 0644)
}

// runRotation fires the correct content type for the current slot, then advances.
func runRotation(client *twitter.Client, seen *content.SeenStore, cfg *config.Config) {
	slot := loadRotationSlot()
	contentType := rotationSlots[slot]
	next := (slot + 1) % len(rotationSlots)
	saveRotationSlot(next)

	fmt.Printf("  rotation slot %d/%d → %s\n", slot+1, len(rotationSlots), contentType)

	switch contentType {
	case "news":
		runNewsOne(client, seen, cfg)
	case "creator":
		runCreator(client, cfg)
	case "meme":
		runMeme(client, seen, cfg, "")
	case "engage":
		runEngagement(client, cfg)
	}
}

// runNewsOne posts a single news article — used inside rotation so we don't dump
// all articles at once and crowd out creator/meme slots.
func runNewsOne(client *twitter.Client, seen *content.SeenStore, cfg *config.Config) {
	articles, err := content.Poll(seen, cfg.MaxArticleAge, cfg.FeedsFile, cfg.Category)
	if err != nil {
		log.Printf("poll error: %v", err)
		return
	}
	if len(articles) == 0 {
		fmt.Println("  no new articles — skipping news slot")
		// Nothing to post this slot — don't burn a creator/meme slot, just skip
		return
	}

	a := articles[0]
	headline := content.FetchAndEngage(a, cfg.GroqAPIKey)
	fmt.Printf("→ [%s] %s\n", a.FeedName, a.Title)
	fmt.Printf("  tweet: %s\n", headline)

	imgPath, err := content.DownloadImage(a.ImageURL)
	if err != nil {
		log.Printf("  image download failed: %v", err)
	}
	if imgPath == "" {
		imgPath, err = content.GeneratePollinationsImage(cfg.GroqAPIKey, headline)
		if err != nil {
			log.Printf("  pollinations failed: %v — text only", err)
		}
	}

	var tweetURL string
	if imgPath != "" {
		tweetURL, err = client.TweetWithMedia(headline, imgPath)
		os.Remove(imgPath)
	} else {
		tweetURL, err = client.Tweet(headline)
	}
	if err != nil {
		log.Printf("tweet failed: %v", err)
		return
	}

	seen.Add(a.Link)
	fmt.Println("  ✓ tweeted")

	if tweetURL != "" {
		time.Sleep(2 * time.Second)
		comment := content.GenerateSelfComment(cfg.GroqAPIKey, headline)
		if err := client.SelfEngage(tweetURL, comment); err != nil {
			log.Printf("  self-engage failed: %v", err)
		}
		time.Sleep(3 * time.Second)
		if _, err := client.ReplyTo(tweetURL, fmt.Sprintf("🔗 Full story: %s", a.Link)); err != nil {
			log.Printf("  link reply failed: %v", err)
		} else {
			fmt.Println("  ✓ link reply posted")
		}
	}
}
