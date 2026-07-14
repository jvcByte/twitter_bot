// Package bot contains the top-level posting modes and orchestration logic.
package bot

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/jvcByte/twitter_bot/internal/config"
	"github.com/jvcByte/twitter_bot/internal/feeds"
	"github.com/jvcByte/twitter_bot/internal/generation"
	"github.com/jvcByte/twitter_bot/internal/images"
	"github.com/jvcByte/twitter_bot/internal/twitter"
)

const seenArticlesPath = "data/seen_articles.json"

// Run starts the bot in the configured mode. If RUN_ONCE is set it runs once
// and exits, otherwise it loops forever on PollInterval.
func Run(cfg *config.Config) {
	client := twitter.NewClient(cfg.TwitterUsername, cfg.TwitterPassword)
	seen := feeds.NewSeenStore(seenArticlesPath)

	feedList, err := feeds.LoadFeeds(cfg.FeedsFile)
	if err != nil {
		log.Fatalf("failed to load feeds: %v", err)
	}

	categoryLabel := cfg.Category
	if categoryLabel == "" {
		categoryLabel = "all"
	}
	fmt.Printf("🚀 Bot started | mode: %s | feeds: %d | category: %s\n",
		cfg.PostMode, len(feedList), categoryLabel)

	runOnce := os.Getenv("RUN_ONCE") == "true"
	if runOnce {
		poll(client, seen, cfg)
		return
	}

	fmt.Printf("⏱  poll every %v | max age %v | tweet delay %v\n\n",
		cfg.PollInterval, cfg.MaxArticleAge, cfg.TweetDelay)
	for {
		poll(client, seen, cfg)
		fmt.Printf("sleeping %v...\n\n", cfg.PollInterval)
		time.Sleep(cfg.PollInterval)
	}
}

func poll(client *twitter.Client, seen *feeds.SeenStore, cfg *config.Config) {
	fmt.Printf("[%s] fetching...\n", time.Now().Format("15:04:05"))
	switch cfg.PostMode {
	case "meme":
		RunMeme(client, seen, cfg, "")
	case "mixed":
		RunMixed(client, seen, cfg)
	case "creator":
		RunCreator(client, cfg)
	case "engage":
		RunEngagement(client, cfg)
	default:
		RunNews(client, seen, cfg)
	}
}

// RunNews polls news feeds and posts articles with images, self-engagement, and link replies.
func RunNews(client *twitter.Client, seen *feeds.SeenStore, cfg *config.Config) {
	articles, err := feeds.Poll(seen, cfg.MaxArticleAge, cfg.FeedsFile, cfg.Category)
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
		tweetURL := postArticle(client, seen, cfg, a)
		if tweetURL != "" {
			tweeted++
		}
		time.Sleep(cfg.TweetDelay)
	}
}

// RunMeme posts a single AI-generated meme post (or thread ~30% of the time).
func RunMeme(client *twitter.Client, seen *feeds.SeenStore, cfg *config.Config, headline string) {
	if cfg.GroqAPIKey == "" {
		log.Printf("GROQ_API_KEY not set — skipping meme post")
		return
	}
	if rand.Intn(10) < 3 {
		runThread(client, cfg, headline)
		return
	}

	post, formatName, err := generation.GenerateMemePost(cfg.GroqAPIKey, headline)
	if err != nil {
		log.Printf("meme generation failed: %v", err)
		return
	}
	fmt.Printf("→ [AI %s] %s\n", formatName, post)

	tweetURL := postWithOptionalImage(client, cfg, post, !generation.IsTextOnly(formatName))
	if tweetURL != "" {
		selfEngage(client, cfg, tweetURL, post)
	}
}

// RunCreator posts owned content grounded in real dev/embedded articles.
func RunCreator(client *twitter.Client, cfg *config.Config) {
	if cfg.GroqAPIKey == "" {
		log.Printf("GROQ_API_KEY not set — skipping creator post")
		return
	}
	if rand.Intn(10) < 3 {
		tweets, err := generation.GenerateCreatorThread(cfg.GroqAPIKey, "")
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
			selfEngage(client, cfg, threadURL, tweets[0])
			return
		}
	}

	post, formatName, err := generation.GenerateCreatorPost(cfg.GroqAPIKey)
	if err != nil {
		log.Printf("creator post failed: %v", err)
		return
	}
	fmt.Printf("→ [creator %s] %s\n", formatName, post)

	tweetURL := postWithOptionalImage(client, cfg, post, !generation.IsCreatorTextOnly(formatName))
	if tweetURL != "" {
		selfEngage(client, cfg, tweetURL, post)
	}
}

// EngagementTopics are the search queries used to find posts to engage with.
var EngagementTopics = []string{
	"PCB design", "embedded systems", "firmware development",
	"KiCad", "STM32", "ESP32",
	"software engineering", "clean code", "Go programming", "Python embedded",
	"#EmbeddedSystems", "#PCBDesign", "#Firmware",
	"#SoftwareEngineering", "#Golang", "AI cybersecurity", "#DevLife",
}

// RunEngagement finds relevant posts and likes, comments, occasionally reposts.
func RunEngagement(client *twitter.Client, cfg *config.Config) {
	fmt.Println("→ [engage] searching for relevant posts...")

	commentFn := func(tweetText string) string {
		if cfg.GroqAPIKey == "" || len(tweetText) < 20 {
			return ""
		}
		comment, err := generation.GenerateEngagementComment(cfg.GroqAPIKey, tweetText)
		if err != nil {
			return ""
		}
		return comment
	}

	n, err := client.EngageWithTopic(EngagementTopics, 5, commentFn, 2)
	if err != nil {
		log.Printf("engagement failed: %v", err)
		return
	}
	fmt.Printf("  ✓ engaged with %d posts\n", n)
}

// RunMixed delegates to the rotation system.
func RunMixed(client *twitter.Client, seen *feeds.SeenStore, cfg *config.Config) {
	runRotation(client, seen, cfg)
}

// ── rotation ────────────────────────────────────────────────────────────────

// rotationSlots defines the 8-slot content cycle for mixed mode.
// At 4 posts/day: 3 news, 2 creator, 1 meme, 2 engage.
var rotationSlots = []string{
	"news", "creator", "engage",
	"news", "meme", "creator",
	"engage", "news",
}

const rotationStatePath = "data/rotation_state.json"

type rotationState struct {
	Slot int `json:"slot"`
}

func loadSlot() int {
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

func saveSlot(slot int) {
	data, _ := json.Marshal(rotationState{Slot: slot})
	os.WriteFile(rotationStatePath, data, 0644) //nolint
}

func runRotation(client *twitter.Client, seen *feeds.SeenStore, cfg *config.Config) {
	slot := loadSlot()
	contentType := rotationSlots[slot]
	saveSlot((slot + 1) % len(rotationSlots))
	fmt.Printf("  rotation slot %d/%d → %s\n", slot+1, len(rotationSlots), contentType)

	switch contentType {
	case "news":
		runNewsOne(client, seen, cfg)
	case "creator":
		RunCreator(client, cfg)
	case "meme":
		RunMeme(client, seen, cfg, "")
	case "engage":
		RunEngagement(client, cfg)
	}
}

func runNewsOne(client *twitter.Client, seen *feeds.SeenStore, cfg *config.Config) {
	articles, err := feeds.Poll(seen, cfg.MaxArticleAge, cfg.FeedsFile, cfg.Category)
	if err != nil {
		log.Printf("poll error: %v", err)
		return
	}
	if len(articles) == 0 {
		fmt.Println("  no new articles — skipping news slot")
		return
	}
	postArticle(client, seen, cfg, articles[0])
}

// ── shared helpers ────────────────────────────────────────────────────────────

func postArticle(client *twitter.Client, seen *feeds.SeenStore, cfg *config.Config, a feeds.Article) string {
	headline := generation.FetchAndEngage(a, cfg.GroqAPIKey)
	fmt.Printf("→ [%s] %s\n", a.FeedName, a.Title)
	fmt.Printf("  tweet: %s\n", headline)

	imgPath, _ := feeds.DownloadImage(a.ImageURL)
	if imgPath == "" {
		var err error
		imgPath, err = images.Pollinations(cfg.GroqAPIKey, headline)
		if err != nil {
			log.Printf("  image failed: %v — text only", err)
		}
	}

	var (
		tweetURL string
		err      error
	)
	if imgPath != "" {
		tweetURL, err = client.TweetWithMedia(headline, imgPath)
		os.Remove(imgPath)
	} else {
		tweetURL, err = client.Tweet(headline)
	}
	if err != nil {
		log.Printf("tweet failed: %v", err)
		return ""
	}
	seen.Add(a.Link)
	fmt.Println("  ✓ tweeted")

	if tweetURL != "" {
		selfEngage(client, cfg, tweetURL, headline)
		time.Sleep(3 * time.Second)
		if _, err := client.ReplyTo(tweetURL, fmt.Sprintf("🔗 Full story: %s", a.Link)); err != nil {
			log.Printf("  link reply failed: %v", err)
		} else {
			fmt.Println("  ✓ link reply posted")
		}
	}
	return tweetURL
}

func postWithOptionalImage(client *twitter.Client, cfg *config.Config, post string, withImage bool) string {
	var tweetURL string
	var err error

	if withImage {
		top, bottom := splitText(post)
		imgPath, imgErr := images.GenerateForPost(cfg.GroqAPIKey, cfg.ImgflipUsername, cfg.ImgflipPassword, top, bottom)
		if imgErr != nil {
			log.Printf("  image failed: %v — text only", imgErr)
		}
		if imgPath != "" {
			tweetURL, err = client.TweetWithMedia(post, imgPath)
			os.Remove(imgPath)
		} else {
			tweetURL, err = client.Tweet(post)
		}
	} else {
		tweetURL, err = client.Tweet(post)
	}

	if err != nil {
		log.Printf("tweet failed: %v", err)
		return ""
	}
	fmt.Println("  ✓ tweeted")
	return tweetURL
}

func selfEngage(client *twitter.Client, cfg *config.Config, tweetURL, postText string) {
	if tweetURL == "" {
		return
	}
	time.Sleep(2 * time.Second)
	comment := generation.GenerateSelfComment(cfg.GroqAPIKey, postText)
	if err := client.SelfEngage(tweetURL, comment); err != nil {
		log.Printf("  self-engage failed: %v", err)
	}
}

func runThread(client *twitter.Client, cfg *config.Config, topic string) {
	tweets, err := generation.GenerateThread(cfg.GroqAPIKey, topic)
	if err != nil {
		log.Printf("thread failed: %v — falling back to single post", err)
		if post, _, err := generation.GenerateMemePost(cfg.GroqAPIKey, topic); err == nil {
			client.Tweet(post) //nolint
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
	selfEngage(client, cfg, threadURL, tweets[0])
}

func splitText(post string) (string, string) {
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
