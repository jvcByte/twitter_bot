package twitter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod"
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

// Tweet posts a text-only tweet
func (c *Client) Tweet(message string) error {
	return c.tweet(message, "")
}

// TweetWithMedia posts a tweet with an attached image
func (c *Client) TweetWithMedia(message, imagePath string) error {
	abs, err := filepath.Abs(imagePath)
	if err != nil {
		return fmt.Errorf("failed to resolve image path: %w", err)
	}
	if _, err := os.Stat(abs); err != nil {
		return fmt.Errorf("image not found at %s: %w", abs, err)
	}
	return c.tweet(message, abs)
}

// tweet is the shared implementation — imagePath is empty for text-only posts
func (c *Client) tweet(message, imagePath string) error {
	if len(message) > 280 {
		return fmt.Errorf("tweet exceeds 280 characters: %d", len(message))
	}

	fmt.Println("Launching browser...")

	browser, page, err := c.launchSession()
	if err != nil {
		return err
	}
	defer browser.MustClose()

	fmt.Println("Session valid, composing tweet...")

	newTweetBtn, err := page.Timeout(timeout).Element(`[data-testid="SideNav_NewTweet_Button"]`)
	if err != nil {
		page.MustScreenshot("debug_compose.png")
		return fmt.Errorf("new tweet button not found: %w", err)
	}
	newTweetBtn.MustEval(`() => this.click()`)
	time.Sleep(3 * time.Second)

	// Attach image if provided
	if imagePath != "" {
		fileInput, err := page.Timeout(timeout).Element(`input[data-testid="fileInput"]`)
		if err != nil {
			page.MustScreenshot("debug_compose.png")
			return fmt.Errorf("media upload input not found: %w", err)
		}
		if err := fileInput.SetFiles([]string{imagePath}); err != nil {
			return fmt.Errorf("failed to attach image: %w", err)
		}
		// Wait for upload to complete
		time.Sleep(4 * time.Second)
	}

	tweetBox, err := page.Timeout(timeout).Element(`[data-testid="tweetTextarea_0"]`)
	if err != nil {
		page.MustScreenshot("debug_compose.png")
		return fmt.Errorf("tweet composer not found: %w", err)
	}
	tweetBox.MustEval(`() => this.focus()`)
	time.Sleep(300 * time.Millisecond)

	if err := page.InsertText(message); err != nil {
		return fmt.Errorf("failed to type message: %w", err)
	}
	time.Sleep(1 * time.Second)
	page.MustScreenshot("debug_typed.png")

	submitBtn, err := page.Timeout(timeout).Element(`[data-testid="tweetButton"]`)
	if err != nil {
		page.MustScreenshot("debug_compose.png")
		return fmt.Errorf("tweet submit button not found: %w", err)
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
			return fmt.Errorf("twitter rejected tweet: %s", text)
		}
	}

	fmt.Println("Tweet posted!")
	fmt.Println("Screenshot saved to tweet_confirmation.png")
	fmt.Println(strings.Repeat("=", 60))
	return nil
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
