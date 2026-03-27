package content

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/mmcdole/gofeed"
)

type RSSFeed struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

func GetRSSPost() (string, error) {
	data, err := os.ReadFile("data/rss_feeds.json")
	if err != nil {
		return "", fmt.Errorf("failed to read RSS feeds: %w", err)
	}

	var feeds []RSSFeed
	if err := json.Unmarshal(data, &feeds); err != nil {
		return "", fmt.Errorf("failed to parse RSS feeds: %w", err)
	}

	rand.Seed(time.Now().UnixNano())
	feed := feeds[rand.Intn(len(feeds))]

	fp := gofeed.NewParser()
	parsedFeed, err := fp.ParseURL(feed.URL)
	if err != nil {
		return "", fmt.Errorf("failed to parse feed %s: %w", feed.Name, err)
	}

	if len(parsedFeed.Items) == 0 {
		return "", fmt.Errorf("no items in feed %s", feed.Name)
	}

	item := parsedFeed.Items[rand.Intn(min(10, len(parsedFeed.Items)))]
	
	post := fmt.Sprintf("📰 %s\n\n%s\n\n#Tech #News", item.Title, item.Link)
	
	if len(post) > 280 {
		maxTitleLen := 280 - len(item.Link) - 20
		post = fmt.Sprintf("📰 %s...\n\n%s\n\n#Tech", item.Title[:maxTitleLen], item.Link)
	}

	return post, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
