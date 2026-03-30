package twitter

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

const timeout = 60 * time.Second

type Client struct {
	username string
	password string
}

func NewClient(username, password, _, _ string) *Client {
	return &Client{
		username: username,
		password: password,
	}
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

	// Use system Chromium if available (e.g. GitHub Actions)
	if path, exists := launcher.LookPath(); exists {
		l = l.Bin(path)
	}

	u := l.MustLaunch()

	browser := rod.New().ControlURL(u).MustConnect()
	defer browser.MustClose()

	// Mask webdriver property to avoid bot detection
	page := browser.MustPage("")
	page.MustEval(`() => {
		Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
	}`)
	page.MustNavigate("https://x.com/login")
	time.Sleep(3 * time.Second)
	page.MustScreenshot("debug_login.png")

	// Wait for username field
	fmt.Println("🔐 Waiting for login page...")
	usernameInput, err := page.Timeout(timeout).Element(`input[autocomplete="username"]`)
	if err != nil {
		page.MustScreenshot("debug_login.png")
		return fmt.Errorf("username field not found: %w", err)
	}
	// Click first, then type slowly to appear human
	usernameInput.MustClick()
	time.Sleep(500 * time.Millisecond)
	usernameInput.MustSelectAllText()
	for _, ch := range c.username {
		usernameInput.MustInput(string(ch))
		time.Sleep(80 * time.Millisecond)
	}
	time.Sleep(500 * time.Millisecond)

	// Press Enter to advance — more reliable than clicking the button
	page.Keyboard.MustType(input.Enter)
	time.Sleep(3 * time.Second)
	page.MustScreenshot("debug_after_username.png")

	// Handle "verify identity" intermediate step (phone/email prompt)
	if el, err := page.Timeout(3 * time.Second).Element(`input[data-testid="ocfEnterTextTextInput"]`); err == nil {
		fmt.Println("🔑 Identity verification step detected...")
		el.MustInput(c.username)
		page.MustElement(`[data-testid="ocfEnterTextNextButton"]`).MustClick()
		time.Sleep(2 * time.Second)
	}

	// Enter password
	fmt.Println("🔑 Entering password...")
	passwordInput, err := page.Timeout(timeout).Element(`input[name="password"]`)
	if err != nil {
		page.MustScreenshot("debug_password.png")
		return fmt.Errorf("password field not found: %w", err)
	}
	passwordInput.MustClick()
	time.Sleep(500 * time.Millisecond)
	for _, ch := range c.password {
		passwordInput.MustInput(string(ch))
		time.Sleep(80 * time.Millisecond)
	}
	time.Sleep(500 * time.Millisecond)
	page.Keyboard.MustType(input.Enter)

	// Wait for home feed
	fmt.Println("⏳ Waiting for home feed...")
	_, err = page.Timeout(20 * time.Second).Element(`[data-testid="SideNav_NewTweet_Button"]`)
	if err != nil {
		page.MustScreenshot("debug_home.png")
		return fmt.Errorf("login failed — home feed not reached (screenshot saved): %w", err)
	}

	fmt.Println("✅ Logged in, composing tweet...")

	page.MustElement(`[data-testid="SideNav_NewTweet_Button"]`).MustClick()
	time.Sleep(1 * time.Second)

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
	submitBtn.MustClick()
	time.Sleep(2 * time.Second)

	page.MustScreenshot("tweet_confirmation.png")
	fmt.Printf("\n✅ Tweet posted!\n")
	fmt.Println("📸 Screenshot saved to tweet_confirmation.png")
	fmt.Println("\n" + strings.Repeat("=", 60))

	return nil
}

func dismissOverlays(page *rod.Page) {
	for _, sel := range []string{
		`[data-testid="confirmationSheetConfirm"]`,
		`[data-testid="BottomBar-close"]`,
	} {
		if el, err := page.Timeout(2 * time.Second).Element(sel); err == nil {
			_ = el.Click(proto.InputMouseButtonLeft, 1)
		}
	}
}
