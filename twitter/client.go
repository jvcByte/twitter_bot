package twitter

import (
	"encoding/json"
	"fmt"
	"os"
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

func (c *Client) Tweet(message string) error {
	if len(message) > 280 {
		return fmt.Errorf("tweet exceeds 280 characters: %d", len(message))
	}

	fmt.Println("🌐 Launching browser...")

	l := launcher.New().
		Headless(true).
		Set("no-sandbox").
		Set("disable-dev-shm-usage").
		Set("disable-blink-features", "AutomationControlled").
		Set("user-agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36").
		Set("window-size", "1280,800")

	if path, exists := launcher.LookPath(); exists {
		l = l.Bin(path)
	}

	browser := rod.New().ControlURL(l.MustLaunch()).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("")
	page.MustEval(`() => Object.defineProperty(navigator, 'webdriver', { get: () => undefined })`)

	// Inject cookies to skip login
	if err := loadCookies(page); err != nil {
		return fmt.Errorf("failed to load cookies: %w", err)
	}

	fmt.Println("🔐 Loading session...")
	page.MustNavigate("https://x.com/home")
	// Wait for network to be idle and page fully loaded
	page.MustWaitLoad()
	time.Sleep(4 * time.Second)
	page.MustScreenshot("debug_home.png")

	// Verify we're logged in
	_, err := page.Timeout(15 * time.Second).Element(`[data-testid="SideNav_NewTweet_Button"]`)
	if err != nil {
		page.MustScreenshot("debug_home.png")
		return fmt.Errorf("session invalid or expired — please refresh cookies.json: %w", err)
	}

	fmt.Println("✅ Session valid, composing tweet...")

	newTweetBtn, err := page.Timeout(timeout).Element(`[data-testid="SideNav_NewTweet_Button"]`)
	if err != nil {
		page.MustScreenshot("debug_compose.png")
		return fmt.Errorf("new tweet button not found: %w", err)
	}
	newTweetBtn.MustEval(`() => this.click()`)
	time.Sleep(2 * time.Second)

	tweetBox, err := page.Timeout(timeout).Element(`[data-testid="tweetTextarea_0"]`)
	if err != nil {
		page.MustScreenshot("debug_compose.png")
		return fmt.Errorf("tweet composer not found: %w", err)
	}
	tweetBox.MustInput(message)
	time.Sleep(500 * time.Millisecond)

	submitBtn, err := page.Timeout(timeout).Element(`[data-testid="tweetButtonInline"]`)
	if err != nil {
		page.MustScreenshot("debug_compose.png")
		return fmt.Errorf("tweet submit button not found: %w", err)
	}
	submitBtn.MustEval(`() => this.click()`)
	time.Sleep(2 * time.Second)

	page.MustScreenshot("tweet_confirmation.png")
	fmt.Println("\n✅ Tweet posted!")
	fmt.Println("📸 Screenshot saved to tweet_confirmation.png")
	fmt.Println("\n" + strings.Repeat("=", 60))

	return nil
}

func loadCookies(page *rod.Page) error {
	// Read cookies from TWITTER_COOKIES env var (JSON string) or cookies.json file
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
