package content

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

// RSSFeed represents a single feed source from rss_feeds.json
type RSSFeed struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Category string `json:"category"`
}

// Article is a fetched news item ready to be tweeted
type Article struct {
	FeedName  string
	Category  string
	Title     string
	Link      string
	ImageURL  string
	Published time.Time
}

// SeenStore persists seen article hashes across restarts to prevent duplicates
type SeenStore struct {
	mu   sync.Mutex
	path string
	seen map[string]bool
}

func NewSeenStore(path string) *SeenStore {
	s := &SeenStore{path: path, seen: make(map[string]bool)}
	s.load()
	return s
}

func (s *SeenStore) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var keys []string
	if err := json.Unmarshal(data, &keys); err != nil {
		return
	}
	for _, k := range keys {
		s.seen[k] = true
	}
}

func (s *SeenStore) save() {
	keys := make([]string, 0, len(s.seen))
	for k := range s.seen {
		keys = append(keys, k)
	}
	// Cap at 10k entries to avoid unbounded growth
	if len(keys) > 10000 {
		sort.Strings(keys)
		keys = keys[len(keys)-10000:]
	}
	data, _ := json.Marshal(keys)
	os.WriteFile(s.path, data, 0644)
}

func (s *SeenStore) Has(link string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.seen[articleHash(link)]
}

func (s *SeenStore) Add(link string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seen[articleHash(link)] = true
	s.save()
}

func articleHash(s string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(s)))[:16]
}

// LoadFeeds reads and parses a feeds JSON file
func LoadFeeds(path string) ([]RSSFeed, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	var feeds []RSSFeed
	if err := json.Unmarshal(data, &feeds); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return feeds, nil
}

// Poll fetches all feeds concurrently and returns unseen articles newer than maxAge, sorted newest first.
// feedsFile is the path to the JSON feeds file. category filters by category when non-empty.
func Poll(seen *SeenStore, maxAge time.Duration, feedsFile, category string) ([]Article, error) {
	feeds, err := LoadFeeds(feedsFile)
	if err != nil {
		return nil, err
	}

	// Filter by category if requested
	if category != "" {
		filtered := feeds[:0]
		for _, f := range feeds {
			if strings.EqualFold(f.Category, category) {
				filtered = append(filtered, f)
			}
		}
		feeds = filtered
	}

	// Shuffle so no single source dominates
	rand.Shuffle(len(feeds), func(i, j int) { feeds[i], feeds[j] = feeds[j], feeds[i] })

	cutoff := time.Now().Add(-maxAge)

	var (
		articles []Article
		mu       sync.Mutex
		wg       sync.WaitGroup
	)

	// Semaphore: max 15 concurrent feed fetches
	sem := make(chan struct{}, 15)
	fp := gofeed.NewParser()
	fp.Client = newHTTPClient()

	for _, f := range feeds {
		wg.Add(1)
		go func(feed RSSFeed) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			parsed, err := fp.ParseURL(feed.URL)
			if err != nil {
				return // silently skip broken feeds
			}

			for _, item := range parsed.Items {
				if item.Link == "" {
					continue
				}

				// Always deduplicate by link for consistency
				if seen.Has(item.Link) {
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

	// Newest first
	sort.Slice(articles, func(i, j int) bool {
		return articles[i].Published.After(articles[j].Published)
	})

	return articles, nil
}

// sanitize strips HTML tags and decodes HTML entities from feed text
var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

func sanitize(s string) string {
	s = htmlTagRe.ReplaceAllString(s, "")  // strip tags
	s = html.UnescapeString(s)             // decode &amp; &lt; &#39; etc.
	return strings.TrimSpace(s)
}

// Format builds a tweet string from an Article, guaranteed <= 280 chars
func Format(a Article) string {
	base := fmt.Sprintf("📰 %s\n\n%s | %s", a.Title, a.FeedName, a.Link)
	if len(base) <= 280 {
		return base
	}

	overhead := len(fmt.Sprintf("📰 ...\n\n%s | %s", a.FeedName, a.Link))
	if overhead >= 280 {
		return fmt.Sprintf("📰 %s", a.Title)[:280]
	}
	maxTitle := 280 - overhead
	return fmt.Sprintf("📰 %s...\n\n%s | %s", a.Title[:maxTitle], a.FeedName, a.Link)
}

// extractImage pulls the best available image URL from a feed item
func extractImage(item *gofeed.Item) string {
	// 1. media:content or media:thumbnail (most common in news feeds)
	if item.Image != nil && item.Image.URL != "" {
		return item.Image.URL
	}
	// 2. enclosures (podcasts/RSS standard)
	for _, enc := range item.Enclosures {
		if strings.HasPrefix(enc.Type, "image/") && enc.URL != "" {
			return enc.URL
		}
	}
	// 3. extensions: media:content
	if media, ok := item.Extensions["media"]; ok {
		if contents, ok := media["content"]; ok {
			for _, c := range contents {
				if url, ok := c.Attrs["url"]; ok && url != "" {
					return url
				}
			}
		}
		if thumbs, ok := media["thumbnail"]; ok {
			for _, t := range thumbs {
				if url, ok := t.Attrs["url"]; ok && url != "" {
					return url
				}
			}
		}
	}
	return ""
}

// DownloadImage downloads an image URL to a temp file and returns the local path.
// Returns ("", nil) if imageURL is empty. Caller is responsible for deleting the file.
func DownloadImage(imageURL string) (string, error) {
	if imageURL == "" {
		return "", nil
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("image download returned %d", resp.StatusCode)
	}

	// Determine extension from Content-Type
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
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("failed to write image: %w", err)
	}

	return f.Name(), nil
}
