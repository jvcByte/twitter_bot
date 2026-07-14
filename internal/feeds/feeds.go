// Package feeds handles RSS/Atom feed polling, article parsing, deduplication,
// and article text fetching for the bot.
package feeds

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"
)

// Feed represents a single RSS/Atom source from a feeds JSON file.
type Feed struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Category string `json:"category"`
}

// Article is a parsed feed item ready for processing.
type Article struct {
	FeedName  string
	Category  string
	Title     string
	Link      string
	ImageURL  string
	Published time.Time
}

// LoadFeeds reads and parses a feeds JSON file.
func LoadFeeds(path string) ([]Feed, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var feeds []Feed
	if err := json.Unmarshal(data, &feeds); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return feeds, nil
}

// Poll fetches all feeds concurrently and returns unseen articles newer than
// maxAge, sorted newest first. feedsFile is the path to the JSON feeds file.
// category filters by category when non-empty (empty or "all" = no filter).
func Poll(seen *SeenStore, maxAge time.Duration, feedsFile, category string) ([]Article, error) {
	feeds, err := LoadFeeds(feedsFile)
	if err != nil {
		return nil, err
	}

	if category != "" && !strings.EqualFold(category, "all") {
		var filtered []Feed
		for _, f := range feeds {
			if strings.EqualFold(f.Category, category) {
				filtered = append(filtered, f)
			}
		}
		feeds = filtered
	}

	rand.Shuffle(len(feeds), func(i, j int) { feeds[i], feeds[j] = feeds[j], feeds[i] })

	cutoff := time.Now().Add(-maxAge)

	var (
		articles []Article
		mu       sync.Mutex
		wg       sync.WaitGroup
	)

	sem := make(chan struct{}, 15) // max 15 concurrent fetches
	fp := gofeed.NewParser()
	fp.Client = &http.Client{Timeout: 15 * time.Second}

	for _, f := range feeds {
		wg.Add(1)
		go func(feed Feed) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			parsed, err := fp.ParseURL(feed.URL)
			if err != nil {
				return // silently skip broken feeds
			}

			for _, item := range parsed.Items {
				if item.Link == "" || seen.Has(item.Link) {
					continue
				}

				pub := time.Now()
				if item.PublishedParsed != nil {
					pub = *item.PublishedParsed
				} else if item.UpdatedParsed != nil {
					pub = *item.UpdatedParsed
				}

				if pub.Before(cutoff) {
					continue
				}

				mu.Lock()
				articles = append(articles, Article{
					FeedName:  feed.Name,
					Category:  feed.Category,
					Title:     sanitize(item.Title),
					Link:      item.Link,
					ImageURL:  extractImage(item),
					Published: pub,
				})
				mu.Unlock()
			}
		}(f)
	}

	wg.Wait()

	sort.Slice(articles, func(i, j int) bool {
		return articles[i].Published.After(articles[j].Published)
	})

	return articles, nil
}

// FetchText downloads a URL and extracts up to 800 chars of plain text (best-effort).
func FetchText(url string) string {
	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; NewsBot/1.0)")
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return ""
	}
	defer resp.Body.Close()

	buf := make([]byte, 32*1024)
	n, _ := io.ReadAtLeast(resp.Body, buf, 1)
	raw := string(buf[:n])

	scriptRe := regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)
	raw = scriptRe.ReplaceAllString(raw, "")
	text := htmlTagRe.ReplaceAllString(raw, " ")
	text = html.UnescapeString(text)
	wsRe := regexp.MustCompile(`\s+`)
	text = strings.TrimSpace(wsRe.ReplaceAllString(text, " "))

	if len(text) > 800 {
		text = text[:800]
	}
	return text
}

// DownloadImage downloads an image URL to a temp file and returns the local path.
// Returns ("", nil) if imageURL is empty. Caller must delete the file.
func DownloadImage(imageURL string) (string, error) {
	if imageURL == "" {
		return "", nil
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("image download returned %d", resp.StatusCode)
	}

	ext := ".jpg"
	ct := resp.Header.Get("Content-Type")
	switch {
	case strings.Contains(ct, "png"):
		ext = ".png"
	case strings.Contains(ct, "gif"):
		ext = ".gif"
	case strings.Contains(ct, "webp"):
		ext = ".webp"
	}

	f, err := os.CreateTemp("", "tweet_img_*"+ext)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("write image: %w", err)
	}
	return f.Name(), nil
}

// FormatHeadline formats an article as a tweet (no link — posted separately as reply).
func FormatHeadline(a Article) string {
	base := fmt.Sprintf("📰 %s\n\n%s", a.Title, a.FeedName)
	if len(base) <= 280 {
		return base
	}
	overhead := len(fmt.Sprintf("📰 ...\n\n%s", a.FeedName))
	if overhead >= 280 {
		return fmt.Sprintf("📰 %s", a.Title)[:280]
	}
	maxTitle := 280 - overhead
	return fmt.Sprintf("📰 %s...\n\n%s", a.Title[:maxTitle], a.FeedName)
}

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

func sanitize(s string) string {
	s = htmlTagRe.ReplaceAllString(s, "")
	return strings.TrimSpace(html.UnescapeString(s))
}

func extractImage(item *gofeed.Item) string {
	if item.Image != nil && item.Image.URL != "" {
		return item.Image.URL
	}
	for _, enc := range item.Enclosures {
		if strings.HasPrefix(enc.Type, "image/") && enc.URL != "" {
			return enc.URL
		}
	}
	if media, ok := item.Extensions["media"]; ok {
		if contents, ok := media["content"]; ok {
			for _, c := range contents {
				if u, ok := c.Attrs["url"]; ok && u != "" {
					return u
				}
			}
		}
		if thumbs, ok := media["thumbnail"]; ok {
			for _, t := range thumbs {
				if u, ok := t.Attrs["url"]; ok && u != "" {
					return u
				}
			}
		}
	}
	return ""
}

// articleHash returns a short hash of a URL for deduplication.
func articleHash(s string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(s)))[:16]
}
