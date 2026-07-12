package twitter

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

const timeout = 30 * time.Second

type Client struct {
	username string
	password string
}

// Cookie represents a browser cookie exported from Cookie-Editor
type Cookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expirationDate"`
	HTTPOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
}

func NewClient(username, password, _, _ string) *Client {
	return &Client{username: username, password: password}
}

// Tweet posts a text-only tweet and returns the URL of the posted tweet.
func (c *Client) Tweet(message string) (string, error) {
	return c.tweetNew(message, "")
}

// TweetWithMedia posts a tweet with an attached image and returns the tweet URL.
func (c *Client) TweetWithMedia(message, imagePath string) (string, error) {
	abs, err := filepath.Abs(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve image path: %w", err)
	}
	if _, err := os.Stat(abs); err != nil {
		return "", fmt.Errorf("image not found at %s: %w", abs, err)
	}
	return c.tweetNew(message, abs)
}

// SelfEngage likes, reposts, and comments on a tweet in a single browser session.
// This boosts engagement velocity in the first few minutes after posting.
// comment is optional — pass empty string to skip commenting.
func (c *Client) SelfEngage(tweetURL, comment string) error {
	if tweetURL == "" {
		return fmt.Errorf("no tweet URL provided")
	}

	fmt.Println("Launching browser for self-engagement...")

	browser, page, err := c.launchSession()
	if err != nil {
		return err
	}
	defer browser.MustClose()

	page.MustNavigate(tweetURL)
	page.MustWaitLoad()
	time.Sleep(3 * time.Second)

	// ── Like ──────────────────────────────────────────────────────────────────
	likeBtn, err := page.Timeout(timeout).Element(`[data-testid="like"]`)
	if err != nil {
		log.Printf("  like button not found: %v", err)
	} else {
		likeBtn.MustEval(`() => this.click()`)
		time.Sleep(1 * time.Second)
		fmt.Println("  ✓ liked")
	}

	// ── Repost ────────────────────────────────────────────────────────────────
	repostBtn, err := page.Timeout(timeout).Element(`[data-testid="retweet"]`)
	if err != nil {
		log.Printf("  repost button not found: %v", err)
	} else {
		repostBtn.MustEval(`() => this.click()`)
		time.Sleep(1 * time.Second)
		// Confirm the repost in the dropdown
		confirmBtn, err := page.Timeout(5 * time.Second).Element(`[data-testid="retweetConfirm"]`)
		if err != nil {
			log.Printf("  repost confirm not found: %v", err)
		} else {
			confirmBtn.MustEval(`() => this.click()`)
			time.Sleep(1 * time.Second)
			fmt.Println("  ✓ reposted")
		}
	}

	// ── Comment ───────────────────────────────────────────────────────────────
	if comment != "" {
		replyBtn, err := page.Timeout(timeout).Element(`[data-testid="reply"]`)
		if err != nil {
			log.Printf("  reply button not found: %v", err)
		} else {
			replyBtn.MustEval(`() => this.click()`)
			time.Sleep(2 * time.Second)

			replyBox, err := page.Timeout(timeout).Element(`[data-testid="tweetTextarea_0"]`)
			if err != nil {
				log.Printf("  reply composer not found: %v", err)
			} else {
				replyBox.MustEval(`() => this.focus()`)
				time.Sleep(300 * time.Millisecond)
				if err := page.InsertText(comment); err != nil {
					log.Printf("  failed to type comment: %v", err)
				} else {
					time.Sleep(1 * time.Second)
					submitBtn, err := page.Timeout(timeout).Element(`[data-testid="tweetButton"]:not([disabled])`)
					if err != nil {
						log.Printf("  comment submit not found: %v", err)
					} else {
						submitBtn.MustEval(`() => this.click()`)
						time.Sleep(3 * time.Second)
						fmt.Println("  ✓ commented")
					}
				}
			}
		}
	}

	fmt.Println("Self-engagement done!")
	return nil
}

// ReplyTo posts a reply to an existing tweet and returns the URL of the new reply.
func (c *Client) ReplyTo(tweetURL, message string) (string, error) {
	if len(message) > 280 {
		return "", fmt.Errorf("reply exceeds 280 characters: %d", len(message))
	}

	fmt.Println("Launching browser for reply...")

	browser, page, err := c.launchSession()
	if err != nil {
		return "", err
	}
	defer browser.MustClose()

	// Navigate to the tweet page
	page.MustNavigate(tweetURL)
	page.MustWaitLoad()
	time.Sleep(3 * time.Second)

	// Click the reply button on the tweet
	replyBtn, err := page.Timeout(timeout).Element(`[data-testid="reply"]`)
	if err != nil {
		page.MustScreenshot("debug_reply.png")
		return "", fmt.Errorf("reply button not found: %w", err)
	}
	replyBtn.MustEval(`() => this.click()`)
	time.Sleep(2 * time.Second)

	replyBox, err := page.Timeout(timeout).Element(`[data-testid="tweetTextarea_0"]`)
	if err != nil {
		page.MustScreenshot("debug_reply.png")
		return "", fmt.Errorf("reply composer not found: %w", err)
	}
	replyBox.MustEval(`() => this.focus()`)
	time.Sleep(500 * time.Millisecond)

	// Move cursor to end — the box may have a pre-filled @mention
	page.MustEval(`() => {
		const sel = window.getSelection();
		const range = document.createRange();
		const el = document.querySelector('[data-testid="tweetTextarea_0"]');
		if (el) { range.selectNodeContents(el); range.collapse(false); sel.removeAllRanges(); sel.addRange(range); }
	}`)
	time.Sleep(200 * time.Millisecond)

	// Add a newline first to push past any pre-filled mention
	if err := page.InsertText("\n" + message); err != nil {
		return "", fmt.Errorf("failed to type reply: %w", err)
	}
	time.Sleep(1 * time.Second)

	submitBtn, err := page.Timeout(timeout).Element(`[data-testid="tweetButton"]:not([disabled]), [data-testid="tweetButtonInline"]:not([disabled])`)
	if err != nil {
		page.MustScreenshot("debug_reply.png")
		return "", fmt.Errorf("reply submit button not found: %w", err)
	}
	submitBtn.MustEval(`() => this.click()`)
	time.Sleep(4 * time.Second)

	// Check for error toast
	if errMsg, _ := page.Timeout(3 * time.Second).Element(`[data-testid="toast"]`); errMsg != nil {
		text, _ := errMsg.Text()
		lower := strings.ToLower(text)
		if strings.Contains(lower, "already said") ||
			strings.Contains(lower, "something went wrong") ||
			strings.Contains(lower, "try again") ||
			strings.Contains(lower, "limit") {
			return "", fmt.Errorf("twitter rejected reply: %s", text)
		}
	}

	replyURL := c.extractPostedTweetURL(page)
	fmt.Printf("Reply posted! url=%s\n", replyURL)
	return replyURL, nil
}

// Thread posts a sequence of tweets as a thread using reply-chaining.
// The first tweet is posted normally; each subsequent tweet replies to the previous one.
// Returns the URL of the first tweet.
func (c *Client) Thread(tweets []string, imagePath string) (string, error) {
	if len(tweets) == 0 {
		return "", fmt.Errorf("no tweets provided")
	}
	for i, t := range tweets {
		if len(t) > 280 {
			return "", fmt.Errorf("tweet %d exceeds 280 characters: %d", i+1, len(t))
		}
	}

	// Post the first tweet
	var firstURL string
	var err error
	if imagePath != "" {
		firstURL, err = c.TweetWithMedia(tweets[0], imagePath)
	} else {
		firstURL, err = c.Tweet(tweets[0])
	}
	if err != nil {
		return "", fmt.Errorf("thread tweet 1 failed: %w", err)
	}

	fmt.Printf("Thread started (%d tweets total), url=%s\n", len(tweets), firstURL)

	if firstURL == "" {
		fmt.Println("  ⚠ no tweet URL returned, cannot chain replies")
		return firstURL, nil
	}

	// Reply to each previous tweet to form a proper chain
	prevURL := firstURL
	for i := 1; i < len(tweets); i++ {
		time.Sleep(3 * time.Second)
		replyURL, err := c.ReplyTo(prevURL, tweets[i])
		if err != nil {
			log.Printf("  thread tweet %d failed: %v — stopping thread", i+1, err)
			break
		}
		fmt.Printf("  ✓ tweet %d/%d\n", i+1, len(tweets))
		if replyURL != "" {
			prevURL = replyURL // chain to the latest reply
		}
	}

	fmt.Println("Thread posted!")
	fmt.Println(strings.Repeat("=", 60))
	return firstURL, nil
}

// tweetNew is the shared implementation for single tweets — imagePath is empty for text-only posts.
// Returns the URL of the posted tweet.
func (c *Client) tweetNew(message, imagePath string) (string, error) {
	if len(message) > 280 {
		return "", fmt.Errorf("tweet exceeds 280 characters: %d", len(message))
	}

	fmt.Println("Launching browser...")

	browser, page, err := c.launchSession()
	if err != nil {
		return "", err
	}
	defer browser.MustClose()

	fmt.Println("Session valid, composing tweet...")

	newTweetBtn, err := page.Timeout(timeout).Element(`[data-testid="SideNav_NewTweet_Button"]`)
	if err != nil {
		page.MustScreenshot("debug_compose.png")
		return "", fmt.Errorf("new tweet button not found: %w", err)
	}
	newTweetBtn.MustEval(`() => this.click()`)
	time.Sleep(3 * time.Second)

	// Attach image if provided
	if imagePath != "" {
		fileInput, err := page.Timeout(timeout).Element(`input[data-testid="fileInput"]`)
		if err != nil {
			page.MustScreenshot("debug_compose.png")
			return "", fmt.Errorf("media upload input not found: %w", err)
		}
		if err := fileInput.SetFiles([]string{imagePath}); err != nil {
			return "", fmt.Errorf("failed to attach image: %w", err)
		}
		c.waitForMediaUpload(page)
	}

	if err := c.typeIntoTextarea(page, `[data-testid="tweetTextarea_0"]`, message); err != nil {
		return "", err
	}

	page.MustScreenshot("debug_typed.png")

	// Wait for the tweet button to be enabled before clicking
	submitBtn, err := page.Timeout(timeout).Element(`[data-testid="tweetButton"]:not([disabled])`)
	if err != nil {
		page.MustScreenshot("debug_compose.png")
		return "", fmt.Errorf("tweet submit button not found or still disabled: %w", err)
	}
	submitBtn.MustEval(`() => this.click()`)
	time.Sleep(4 * time.Second)

	page.MustScreenshot("tweet_confirmation.png")

	// Check for error toast
	if errMsg, _ := page.Timeout(3 * time.Second).Element(`[data-testid="toast"]`); errMsg != nil {
		text, _ := errMsg.Text()
		lower := strings.ToLower(text)
		if strings.Contains(lower, "already said") ||
			strings.Contains(lower, "something went wrong") ||
			strings.Contains(lower, "try again") ||
			strings.Contains(lower, "limit") {
			return "", fmt.Errorf("twitter rejected tweet: %s", text)
		}
	}

	tweetURL := c.extractPostedTweetURL(page)
	fmt.Println("Tweet posted!")
	fmt.Println("Screenshot saved to tweet_confirmation.png")
	fmt.Println(strings.Repeat("=", 60))
	return tweetURL, nil
}

// waitForMediaUpload waits until Twitter's media upload progress bar disappears,
// indicating the upload is complete and the tweet button will be enabled.
func (c *Client) waitForMediaUpload(page *rod.Page) {
	// Wait up to 30s for the progress bar to appear then disappear
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		// Upload is done when there's no active progress indicator
		uploading, _ := page.Elements(`[data-testid="attachments"] [role="progressbar"]`)
		if len(uploading) == 0 {
			// Also check the generic progress bar is gone
			progress, _ := page.Elements(`[role="progressbar"]`)
			if len(progress) == 0 {
				break
			}
		}
	}
	// Small buffer after upload completes
	time.Sleep(1 * time.Second)
}
func (c *Client) typeIntoTextarea(page *rod.Page, selector, text string) error {
	box, err := page.Timeout(timeout).Element(selector)
	if err != nil {
		page.MustScreenshot("debug_compose.png")
		return fmt.Errorf("textarea %q not found: %w", selector, err)
	}
	box.MustEval(`() => this.focus()`)
	time.Sleep(300 * time.Millisecond)
	if err := page.InsertText(text); err != nil {
		return fmt.Errorf("failed to type into %q: %w", selector, err)
	}
	time.Sleep(500 * time.Millisecond)
	return nil
}

// extractPostedTweetURL tries to find the URL of the just-posted tweet.
// After posting, X stays on the home feed — we look for the newest tweet link
// in the timeline that belongs to our account.
func (c *Client) extractPostedTweetURL(page *rod.Page) string {
	// Give the feed a moment to update
	time.Sleep(2 * time.Second)

	// Look for tweet permalink links in the timeline — format: /username/status/ID
	// Try the page's current URL first (sometimes X does navigate)
	info, _ := page.Info()
	if info != nil && strings.Contains(info.URL, "/status/") {
		return info.URL
	}

	// Find all status links on the page and return the first one matching our username
	links, _ := page.Elements(`a[href*="/status/"]`)
	for _, link := range links {
		href, _ := link.Attribute("href")
		if href == nil {
			continue
		}
		h := *href
		// Must be a status link for our account
		if strings.Contains(strings.ToLower(h), strings.ToLower(c.username)) &&
			strings.Contains(h, "/status/") {
			if strings.HasPrefix(h, "/") {
				return "https://x.com" + h
			}
			return h
		}
	}

	// Fallback: grab any status link — the newest tweet is usually first
	for _, link := range links {
		href, _ := link.Attribute("href")
		if href == nil {
			continue
		}
		h := *href
		if strings.Contains(h, "/status/") {
			if strings.HasPrefix(h, "/") {
				return "https://x.com" + h
			}
			return h
		}
	}

	return ""
}

// launchSession starts a browser, loads cookies, and verifies the session
func (c *Client) launchSession() (*rod.Browser, *rod.Page, error) {
	l := launcher.New().
		Headless(true).
		Set("no-sandbox").
		Set("disable-setuid-sandbox").
		Set("disable-dev-shm-usage").
		Set("disable-gpu").
		Set("disable-blink-features", "AutomationControlled").
		Set("disable-extensions").
		Set("no-first-run").
		Set("user-agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36").
		Set("window-size", "1280,800")

	if path := snapChromium(); path != "" {
		l = l.Bin(path)
	} else if path, exists := launcher.LookPath(); exists {
		l = l.Bin(path)
	} else if path := rodCachedBrowser(); path != "" {
		l = l.Bin(path)
	} else {
		return nil, nil, fmt.Errorf("no Chromium/Chrome binary found — install google-chrome-stable")
	}

	u, err := l.Launch()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	browser := rod.New().ControlURL(u).MustConnect()
	page := browser.MustPage("")
	page.MustEval(`() => Object.defineProperty(navigator, 'webdriver', { get: () => undefined })`)

	if err := loadCookies(page); err != nil {
		browser.MustClose()
		return nil, nil, fmt.Errorf("failed to load cookies: %w", err)
	}

	fmt.Println("Loading session...")
	page.MustNavigate("https://x.com/home")
	page.MustWaitLoad()
	time.Sleep(4 * time.Second)
	page.MustScreenshot("debug_home.png")

	_, err = page.Timeout(15 * time.Second).Element(`[data-testid="SideNav_NewTweet_Button"]`)
	if err != nil {
		page.MustScreenshot("debug_home.png")
		browser.MustClose()
		return nil, nil, fmt.Errorf("session invalid or expired — please refresh cookies: %w", err)
	}

	return browser, page, nil
}

// snapChromium returns the first available system Chrome/Chromium binary
func snapChromium() string {
	for _, p := range []string{
		"/usr/bin/google-chrome-stable",
		"/usr/bin/google-chrome",
		"/usr/bin/chromium",
		"/usr/bin/chromium-browser",
		"/snap/bin/chromium",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// rodCachedBrowser returns the path to rod's downloaded Chromium if present
func rodCachedBrowser() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	cacheDir := home + "/.cache/rod/browser"
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		for _, bin := range []string{"/chrome-linux/chrome", "/chrome"} {
			p := cacheDir + "/" + e.Name() + bin
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

func loadCookies(page *rod.Page) error {
	var data []byte
	if raw := os.Getenv("TWITTER_COOKIES"); raw != "" {
		data = []byte(raw)
	} else {
		var err error
		data, err = os.ReadFile("cookies.json")
		if err != nil {
			return fmt.Errorf("cookies.json not found and TWITTER_COOKIES env not set: %w", err)
		}
	}

	var cookies []Cookie
	if err := json.Unmarshal(data, &cookies); err != nil {
		return fmt.Errorf("failed to parse cookies: %w", err)
	}

	for _, c := range cookies {
		if !strings.Contains(c.Domain, "twitter.com") && !strings.Contains(c.Domain, "x.com") {
			continue
		}
		cookie := proto.NetworkCookieParam{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
		}
		if c.Expires > 0 {
			cookie.Expires = proto.TimeSinceEpoch(c.Expires)
		}
		if err := page.SetCookies([]*proto.NetworkCookieParam{&cookie}); err != nil {
			return err
		}
	}
	return nil
}

// EngageWithTopic searches X for a topic, then likes and optionally comments/reposts
// on relevant posts from other users in a single browser session.
func (c *Client) EngageWithTopic(topics []string, maxPosts int, commentFn func(string) string, repostChance int) (n int, err error) {
	if len(topics) == 0 || maxPosts == 0 {
		return 0, nil
	}

	defer func() {
		if r := recover(); r != nil {
			log.Printf("  engagement recovered from panic: %v", r)
			err = nil
		}
	}()

	topic := topics[time.Now().UnixNano()%int64(len(topics))]
	fmt.Printf("Launching browser for social engagement (topic: %q)...\n", topic)

	browser, page, launchErr := c.launchSession()
	if launchErr != nil {
		return 0, launchErr
	}
	defer browser.MustClose()

	searchURL := "https://x.com/search?q=" + urlEncode(topic) + "&src=typed_query&f=live"
	page.MustNavigate(searchURL)
	page.MustWaitLoad()
	time.Sleep(4 * time.Second)

	// Collect tweet URLs first — then navigate to each one individually.
	// This avoids stale element references caused by page navigation.
	var tweetURLs []string
	seen := map[string]bool{}

	for scroll := 0; scroll < 4 && len(tweetURLs) < maxPosts*2; scroll++ {
		articles, _ := page.Elements(`article[data-testid="tweet"]`)
		for _, article := range articles {
			links, _ := article.Elements(`a[href*="/status/"]`)
			for _, l := range links {
				href, _ := l.Attribute("href")
				if href == nil || !strings.Contains(*href, "/status/") {
					continue
				}
				path := *href
				if seen[path] {
					continue
				}
				if strings.Contains(strings.ToLower(path), strings.ToLower(c.username)) {
					continue
				}
				seen[path] = true
				tweetURLs = append(tweetURLs, "https://x.com"+path)
				break
			}
		}
		if len(tweetURLs) < maxPosts*2 {
			page.MustEval(`() => window.scrollBy(0, window.innerHeight * 2)`)
			time.Sleep(2 * time.Second)
		}
	}

	// Now engage with each tweet by navigating to its page
	engaged := 0
	for _, tweetURL := range tweetURLs {
		if engaged >= maxPosts {
			break
		}

		page.MustNavigate(tweetURL)
		page.MustWaitLoad()
		time.Sleep(3 * time.Second)

		// ── Like ──────────────────────────────────────────────────────────────
		likeBtn, err := page.Timeout(timeout).Element(`[data-testid="like"]`)
		if err != nil {
			log.Printf("  like not found on %s: %v", tweetURL, err)
			continue
		}
		likeBtn.MustEval(`() => this.click()`)
		time.Sleep(800 * time.Millisecond)
		fmt.Printf("  ✓ liked: %s\n", tweetURL)

		// ── Comment (always) ─────────────────────────────────────────────────
		if commentFn != nil {
			tweetText := ""
			if textEl, err := page.Timeout(5*time.Second).Element(`[data-testid="tweetText"]`); err == nil {
				tweetText, _ = textEl.Text()
			}
			if tweetText != "" {
				comment := commentFn(tweetText)
				if comment != "" {
					replyBtn, err := page.Timeout(timeout).Element(`[data-testid="reply"]`)
					if err == nil {
						replyBtn.MustEval(`() => this.click()`)
						time.Sleep(2 * time.Second)
						if replyBox, err := page.Timeout(timeout).Element(`[data-testid="tweetTextarea_0"]`); err == nil {
							replyBox.MustEval(`() => this.focus()`)
							time.Sleep(300 * time.Millisecond)
							if err := page.InsertText(comment); err == nil {
								time.Sleep(800 * time.Millisecond)
								if submitBtn, err := page.Timeout(timeout).Element(`[data-testid="tweetButton"]:not([disabled])`); err == nil {
									submitBtn.MustEval(`() => this.click()`)
									time.Sleep(2 * time.Second)
									fmt.Printf("  ✓ commented on %s\n", tweetURL)
								}
							}
						}
					}
				}
			}
		}

		// ── Repost (probabilistic) ────────────────────────────────────────────
		if repostChance > 0 && int(time.Now().UnixNano()%10) < repostChance {
			if repostBtn, err := page.Timeout(timeout).Element(`[data-testid="retweet"]`); err == nil {
				repostBtn.MustEval(`() => this.click()`)
				time.Sleep(1 * time.Second)
				if confirmBtn, err := page.Timeout(5*time.Second).Element(`[data-testid="retweetConfirm"]`); err == nil {
					confirmBtn.MustEval(`() => this.click()`)
					time.Sleep(1 * time.Second)
					fmt.Printf("  ✓ reposted: %s\n", tweetURL)
				}
			}
		}

		engaged++
		time.Sleep(1500 * time.Millisecond)
	}

	fmt.Printf("Social engagement done: %d posts engaged\n", engaged)
	return engaged, nil
}

// replyInSession is kept for potential reuse but EngageWithTopic now uses ReplyTo instead.
func (c *Client) replyInSession(page *rod.Page, article *rod.Element, comment string) (string, error) {
	replyBtn, err := article.Element(`[data-testid="reply"]`)
	if err != nil {
		return "", fmt.Errorf("reply button not found")
	}
	replyBtn.MustEval(`() => this.click()`)
	time.Sleep(2 * time.Second)

	replyBox, err := page.Timeout(timeout).Element(`[data-testid="tweetTextarea_0"]`)
	if err != nil {
		return "", fmt.Errorf("reply composer not found")
	}
	replyBox.MustEval(`() => this.focus()`)
	time.Sleep(300 * time.Millisecond)
	if err := page.InsertText(comment); err != nil {
		return "", fmt.Errorf("type failed: %w", err)
	}
	time.Sleep(800 * time.Millisecond)

	submitBtn, err := page.Timeout(timeout).Element(`[data-testid="tweetButton"]:not([disabled])`)
	if err != nil {
		// Close the composer and bail
		page.Keyboard.Press(input.Escape) //nolint
		return "", fmt.Errorf("submit button not found")
	}
	submitBtn.MustEval(`() => this.click()`)
	time.Sleep(3 * time.Second)
	return "", nil
}

// urlEncode encodes a string for use in a URL query parameter.
func urlEncode(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9',
			r == '-', r == '_', r == '.', r == '~':
			b.WriteRune(r)
		case r == ' ':
			b.WriteString("%20")
		default:
			b.WriteString(fmt.Sprintf("%%%02X", r))
		}
	}
	return b.String()
}
