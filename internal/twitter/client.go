// Package twitter provides a headless-browser client for posting and engaging on X (Twitter).
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

const sessionTimeout = 30 * time.Second

// Client is a stateless Twitter automation client backed by a headless browser.
type Client struct {
	username string
	password string
}

// Cookie represents a browser cookie exported from Cookie-Editor.
type Cookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expirationDate"`
	HTTPOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
}

// NewClient creates a new Twitter client for the given account.
func NewClient(username, password string) *Client {
	return &Client{username: username, password: password}
}

// Tweet posts a text-only tweet and returns its URL.
func (c *Client) Tweet(message string) (string, error) {
	return c.tweetNew(message, "")
}

// TweetWithMedia posts a tweet with an attached image and returns its URL.
func (c *Client) TweetWithMedia(message, imagePath string) (string, error) {
	abs, err := filepath.Abs(imagePath)
	if err != nil {
		return "", fmt.Errorf("resolve image path: %w", err)
	}
	if _, err := os.Stat(abs); err != nil {
		return "", fmt.Errorf("image not found at %s: %w", abs, err)
	}
	return c.tweetNew(message, abs)
}

// ReplyTo posts a reply to an existing tweet and returns the reply URL.
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

	page.MustNavigate(tweetURL)
	page.MustWaitLoad()
	time.Sleep(3 * time.Second)

	replyBtn, err := page.Timeout(sessionTimeout).Element(`[data-testid="reply"]`)
	if err != nil {
		page.MustScreenshot("debug_reply.png")
		return "", fmt.Errorf("reply button not found: %w", err)
	}
	replyBtn.MustEval(`() => this.click()`)
	time.Sleep(2 * time.Second)

	replyBox, err := page.Timeout(sessionTimeout).Element(`[data-testid="tweetTextarea_0"]`)
	if err != nil {
		page.MustScreenshot("debug_reply.png")
		return "", fmt.Errorf("reply composer not found: %w", err)
	}
	replyBox.MustEval(`() => this.focus()`)
	time.Sleep(500 * time.Millisecond)

	// Move cursor to end to bypass any pre-filled @mention
	page.MustEval(`() => {
		const sel = window.getSelection();
		const range = document.createRange();
		const el = document.querySelector('[data-testid="tweetTextarea_0"]');
		if (el) { range.selectNodeContents(el); range.collapse(false); sel.removeAllRanges(); sel.addRange(range); }
	}`)
	time.Sleep(200 * time.Millisecond)

	if err := page.InsertText("\n" + message); err != nil {
		return "", fmt.Errorf("type reply: %w", err)
	}
	time.Sleep(1 * time.Second)

	submitBtn, err := page.Timeout(sessionTimeout).Element(
		`[data-testid="tweetButton"]:not([disabled]), [data-testid="tweetButtonInline"]:not([disabled])`)
	if err != nil {
		page.MustScreenshot("debug_reply.png")
		return "", fmt.Errorf("reply submit not found: %w", err)
	}
	submitBtn.MustEval(`() => this.click()`)
	time.Sleep(4 * time.Second)

	if errMsg, _ := page.Timeout(3 * time.Second).Element(`[data-testid="toast"]`); errMsg != nil {
		if text, _ := errMsg.Text(); isErrorToast(text) {
			return "", fmt.Errorf("twitter rejected reply: %s", text)
		}
	}

	url := c.extractPostedTweetURL(page)
	fmt.Printf("Reply posted! url=%s\n", url)
	return url, nil
}

// Thread posts a slice of tweets as a reply chain. Returns the URL of the first tweet.
func (c *Client) Thread(tweets []string, imagePath string) (string, error) {
	if len(tweets) == 0 {
		return "", fmt.Errorf("no tweets provided")
	}
	var firstURL string
	var err error
	if imagePath != "" {
		firstURL, err = c.TweetWithMedia(tweets[0], imagePath)
	} else {
		firstURL, err = c.Tweet(tweets[0])
	}
	if err != nil {
		return "", fmt.Errorf("thread tweet 1: %w", err)
	}
	fmt.Printf("Thread started (%d tweets), url=%s\n", len(tweets), firstURL)
	if firstURL == "" {
		return firstURL, nil
	}

	prevURL := firstURL
	for i := 1; i < len(tweets); i++ {
		time.Sleep(3 * time.Second)
		replyURL, err := c.ReplyTo(prevURL, tweets[i])
		if err != nil {
			log.Printf("  thread tweet %d failed: %v — stopping", i+1, err)
			break
		}
		fmt.Printf("  ✓ tweet %d/%d\n", i+1, len(tweets))
		if replyURL != "" {
			prevURL = replyURL
		}
	}
	fmt.Println("Thread posted!")
	fmt.Println(strings.Repeat("=", 60))
	return firstURL, nil
}

// SelfEngage likes, reposts, and optionally comments on a tweet.
// Used to boost engagement velocity immediately after posting.
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

	if likeBtn, err := page.Timeout(sessionTimeout).Element(`[data-testid="like"]`); err == nil {
		likeBtn.MustEval(`() => this.click()`)
		time.Sleep(1 * time.Second)
		fmt.Println("  ✓ liked")
	}

	if repostBtn, err := page.Timeout(sessionTimeout).Element(`[data-testid="retweet"]`); err == nil {
		repostBtn.MustEval(`() => this.click()`)
		time.Sleep(1 * time.Second)
		if confirmBtn, err := page.Timeout(5 * time.Second).Element(`[data-testid="retweetConfirm"]`); err == nil {
			confirmBtn.MustEval(`() => this.click()`)
			time.Sleep(1 * time.Second)
			fmt.Println("  ✓ reposted")
		}
	}

	if comment != "" {
		if replyBtn, err := page.Timeout(sessionTimeout).Element(`[data-testid="reply"]`); err == nil {
			replyBtn.MustEval(`() => this.click()`)
			time.Sleep(2 * time.Second)
			if replyBox, err := page.Timeout(sessionTimeout).Element(`[data-testid="tweetTextarea_0"]`); err == nil {
				replyBox.MustEval(`() => this.focus()`)
				time.Sleep(300 * time.Millisecond)
				if err := page.InsertText(comment); err == nil {
					time.Sleep(1 * time.Second)
					if submitBtn, err := page.Timeout(sessionTimeout).Element(`[data-testid="tweetButton"]:not([disabled])`); err == nil {
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

// EngageWithTopic searches X for a topic and likes/comments/reposts relevant posts.
// commentFn is called with each tweet's text and returns the comment to post (or "" to skip).
// repostChance is 0–10; e.g. 2 = ~20% chance of reposting each post.
func (c *Client) EngageWithTopic(topics []string, maxPosts int, commentFn func(string) string, repostChance int) (n int, err error) {
	if len(topics) == 0 || maxPosts == 0 {
		return 0, nil
	}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("  engagement recovered from panic: %v", r)
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

	// Phase 1: collect tweet URLs (no interactions — avoids stale elements)
	var tweetURLs []string
	seen := map[string]bool{}
	for scroll := 0; scroll < 4 && len(tweetURLs) < maxPosts*2; scroll++ {
		for _, article := range mustElements(page, `article[data-testid="tweet"]`) {
			for _, l := range mustElements(article, `a[href*="/status/"]`) {
				href, _ := l.Attribute("href")
				if href == nil || !strings.Contains(*href, "/status/") {
					continue
				}
				path := *href
				if seen[path] || strings.Contains(strings.ToLower(path), strings.ToLower(c.username)) {
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

	// Phase 2: navigate to each tweet and engage
	for _, tweetURL := range tweetURLs {
		if n >= maxPosts {
			break
		}
		page.MustNavigate(tweetURL)
		page.MustWaitLoad()
		time.Sleep(3 * time.Second)

		likeBtn, err := page.Timeout(sessionTimeout).Element(`[data-testid="like"]`)
		if err != nil {
			continue
		}
		likeBtn.MustEval(`() => this.click()`)
		time.Sleep(800 * time.Millisecond)
		fmt.Printf("  ✓ liked: %s\n", tweetURL)

		if commentFn != nil {
			tweetText := ""
			if el, err := page.Timeout(5 * time.Second).Element(`[data-testid="tweetText"]`); err == nil {
				tweetText, _ = el.Text()
			}
			if tweetText != "" {
				if comment := commentFn(tweetText); comment != "" {
					if replyBtn, err := page.Timeout(sessionTimeout).Element(`[data-testid="reply"]`); err == nil {
						replyBtn.MustEval(`() => this.click()`)
						time.Sleep(2 * time.Second)
						if replyBox, err := page.Timeout(sessionTimeout).Element(`[data-testid="tweetTextarea_0"]`); err == nil {
							replyBox.MustEval(`() => this.focus()`)
							time.Sleep(300 * time.Millisecond)
							if err := page.InsertText(comment); err == nil {
								time.Sleep(800 * time.Millisecond)
								if submitBtn, err := page.Timeout(sessionTimeout).Element(`[data-testid="tweetButton"]:not([disabled])`); err == nil {
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

		if repostChance > 0 && int(time.Now().UnixNano()%10) < repostChance {
			if repostBtn, err := page.Timeout(sessionTimeout).Element(`[data-testid="retweet"]`); err == nil {
				repostBtn.MustEval(`() => this.click()`)
				time.Sleep(1 * time.Second)
				if confirmBtn, err := page.Timeout(5 * time.Second).Element(`[data-testid="retweetConfirm"]`); err == nil {
					confirmBtn.MustEval(`() => this.click()`)
					time.Sleep(1 * time.Second)
					fmt.Printf("  ✓ reposted: %s\n", tweetURL)
				}
			}
		}
		n++
		time.Sleep(1500 * time.Millisecond)
	}

	fmt.Printf("Social engagement done: %d posts engaged\n", n)
	return n, nil
}

// ── internal helpers ────────────────────────────────────────────────────────

func (c *Client) tweetNew(message, imagePath string) (string, error) {
	if len(message) > 280 {
		return "", fmt.Errorf("tweet exceeds 280 chars: %d", len(message))
	}
	fmt.Println("Launching browser...")
	browser, page, err := c.launchSession()
	if err != nil {
		return "", err
	}
	defer browser.MustClose()
	fmt.Println("Session valid, composing tweet...")

	newTweetBtn, err := page.Timeout(sessionTimeout).Element(`[data-testid="SideNav_NewTweet_Button"]`)
	if err != nil {
		page.MustScreenshot("debug_compose.png")
		return "", fmt.Errorf("new tweet button not found: %w", err)
	}
	newTweetBtn.MustEval(`() => this.click()`)
	time.Sleep(3 * time.Second)

	if imagePath != "" {
		fileInput, err := page.Timeout(sessionTimeout).Element(`input[data-testid="fileInput"]`)
		if err != nil {
			return "", fmt.Errorf("media upload input not found: %w", err)
		}
		if err := fileInput.SetFiles([]string{imagePath}); err != nil {
			return "", fmt.Errorf("attach image: %w", err)
		}
		c.waitForUpload(page)
	}

	if err := c.typeText(page, `[data-testid="tweetTextarea_0"]`, message); err != nil {
		return "", err
	}
	page.MustScreenshot("debug_typed.png")

	submitBtn, err := page.Timeout(sessionTimeout).Element(`[data-testid="tweetButton"]:not([disabled])`)
	if err != nil {
		return "", fmt.Errorf("tweet button not found: %w", err)
	}
	submitBtn.MustEval(`() => this.click()`)
	time.Sleep(4 * time.Second)
	page.MustScreenshot("tweet_confirmation.png")

	if errMsg, _ := page.Timeout(3 * time.Second).Element(`[data-testid="toast"]`); errMsg != nil {
		if text, _ := errMsg.Text(); isErrorToast(text) {
			return "", fmt.Errorf("twitter rejected tweet: %s", text)
		}
	}

	url := c.extractPostedTweetURL(page)
	fmt.Println("Tweet posted!")
	fmt.Println(strings.Repeat("=", 60))
	return url, nil
}

func (c *Client) waitForUpload(page *rod.Page) {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		uploading, _ := page.Elements(`[data-testid="attachments"] [role="progressbar"]`)
		if len(uploading) == 0 {
			if progress, _ := page.Elements(`[role="progressbar"]`); len(progress) == 0 {
				break
			}
		}
	}
	time.Sleep(1 * time.Second)
}

func (c *Client) typeText(page *rod.Page, selector, text string) error {
	box, err := page.Timeout(sessionTimeout).Element(selector)
	if err != nil {
		page.MustScreenshot("debug_compose.png")
		return fmt.Errorf("textarea %q not found: %w", selector, err)
	}
	box.MustEval(`() => this.focus()`)
	time.Sleep(300 * time.Millisecond)
	if err := page.InsertText(text); err != nil {
		return fmt.Errorf("type into %q: %w", selector, err)
	}
	time.Sleep(500 * time.Millisecond)
	return nil
}

func (c *Client) extractPostedTweetURL(page *rod.Page) string {
	time.Sleep(2 * time.Second)
	if info, _ := page.Info(); info != nil && strings.Contains(info.URL, "/status/") {
		return info.URL
	}
	links, _ := page.Elements(`a[href*="/status/"]`)
	for _, link := range links {
		href, _ := link.Attribute("href")
		if href == nil {
			continue
		}
		h := *href
		if strings.Contains(strings.ToLower(h), strings.ToLower(c.username)) && strings.Contains(h, "/status/") {
			return toAbsURL(h)
		}
	}
	for _, link := range links {
		href, _ := link.Attribute("href")
		if href == nil {
			continue
		}
		if strings.Contains(*href, "/status/") {
			return toAbsURL(*href)
		}
	}
	return ""
}

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

	for _, p := range []string{
		"/usr/bin/google-chrome-stable", "/usr/bin/google-chrome",
		"/usr/bin/chromium", "/usr/bin/chromium-browser", "/snap/bin/chromium",
	} {
		if _, err := os.Stat(p); err == nil {
			l = l.Bin(p)
			break
		}
	}
	if _, exists := launcher.LookPath(); exists {
		// already set or will be found automatically
	}
	if home, err := os.UserHomeDir(); err == nil {
		cacheDir := home + "/.cache/rod/browser"
		if entries, err := os.ReadDir(cacheDir); err == nil {
			for _, e := range entries {
				for _, bin := range []string{"/chrome-linux/chrome", "/chrome"} {
					p := cacheDir + "/" + e.Name() + bin
					if _, err := os.Stat(p); err == nil {
						l = l.Bin(p)
					}
				}
			}
		}
	}

	u, err := l.Launch()
	if err != nil {
		return nil, nil, fmt.Errorf("launch browser: %w", err)
	}

	browser := rod.New().ControlURL(u).MustConnect()
	page := browser.MustPage("")
	page.MustEval(`() => Object.defineProperty(navigator, 'webdriver', { get: () => undefined })`)

	fmt.Println("Loading session...")

	// Try cookies first; fall back to username/password login
	if err := loadCookies(page); err != nil {
		fmt.Printf("  cookies unavailable (%v) — logging in with username/password\n", err)
		if loginErr := c.login(page); loginErr != nil {
			browser.MustClose()
			return nil, nil, fmt.Errorf("login failed: %w", loginErr)
		}
	} else {
		page.MustNavigate("https://x.com/home")
		page.MustWaitLoad()
		time.Sleep(4 * time.Second)
		page.MustScreenshot("debug_home.png")

		// If cookies are stale the home button won't be there — fall back to login
		if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="SideNav_NewTweet_Button"]`); err != nil {
			fmt.Println("  session stale — logging in with username/password")
			if loginErr := c.login(page); loginErr != nil {
				browser.MustClose()
				return nil, nil, fmt.Errorf("login failed: %w", loginErr)
			}
		}
	}

	// Final session check
	if _, err := page.Timeout(15 * time.Second).Element(`[data-testid="SideNav_NewTweet_Button"]`); err != nil {
		browser.MustClose()
		return nil, nil, fmt.Errorf("session invalid after login attempt: %w", err)
	}
	return browser, page, nil
}

// login performs a full username/password login on x.com.
// Used as fallback when cookies are absent or expired.
func (c *Client) login(page *rod.Page) error {
	if c.username == "" || c.password == "" {
		return fmt.Errorf("TWITTER_USERNAME and TWITTER_PASSWORD must be set for password login")
	}

	fmt.Println("  logging in with username/password...")
	page.MustNavigate("https://x.com/i/flow/login")
	page.MustWaitLoad()
	time.Sleep(4 * time.Second)

	// Username field — X renders this as a generic text input in a React form.
	// Try multiple selectors to handle different X UI versions.
	usernameInput, err := findFirstElement(page, 20*time.Second,
		`input[autocomplete="username"]`,
		`input[name="text"]`,
		`input[type="text"]`,
	)
	if err != nil {
		page.MustScreenshot("debug_login.png")
		return fmt.Errorf("username field not found: %w", err)
	}
	usernameInput.MustEval(`() => this.focus()`)
	time.Sleep(400 * time.Millisecond)
	if err := page.InsertText(c.username); err != nil {
		return fmt.Errorf("type username: %w", err)
	}
	time.Sleep(600 * time.Millisecond)

	// Click Next — try button text first, then data-testid
	nextBtn, err := findFirstElement(page, 8*time.Second,
		`[data-testid="LoginForm_Login_Button"]`,
		`div[role="button"]:has-text("Next")`,
		`span:has-text("Next")`,
	)
	if err != nil {
		page.Keyboard.Press(input.Enter) //nolint
	} else {
		nextBtn.MustEval(`() => this.click()`)
	}
	time.Sleep(3 * time.Second)

	// X sometimes asks for email/phone verification between username and password
	if verify, _ := page.Timeout(5 * time.Second).Element(`input[data-testid="ocfEnterTextTextInput"]`); verify != nil {
		fmt.Println("  ⚠ verification prompt — entering username again")
		verify.MustEval(`() => this.focus()`)
		time.Sleep(300 * time.Millisecond)
		page.InsertText(c.username) //nolint
		time.Sleep(400 * time.Millisecond)
		if verifyNext, err := page.Timeout(5 * time.Second).Element(`[data-testid="ocfEnterTextNextButton"]`); err == nil {
			verifyNext.MustEval(`() => this.click()`)
			time.Sleep(2 * time.Second)
		}
	}

	// Password field
	passwordInput, err := findFirstElement(page, 15*time.Second,
		`input[name="password"]`,
		`input[type="password"]`,
		`input[autocomplete="current-password"]`,
	)
	if err != nil {
		page.MustScreenshot("debug_login.png")
		return fmt.Errorf("password field not found: %w", err)
	}
	passwordInput.MustEval(`() => this.focus()`)
	time.Sleep(400 * time.Millisecond)
	if err := page.InsertText(c.password); err != nil {
		return fmt.Errorf("type password: %w", err)
	}
	time.Sleep(600 * time.Millisecond)

	// Click Log in
	loginBtn, err := findFirstElement(page, 8*time.Second,
		`[data-testid="LoginForm_Login_Button"]`,
		`div[role="button"]:has-text("Log in")`,
		`span:has-text("Log in")`,
	)
	if err != nil {
		page.Keyboard.Press(input.Enter) //nolint
	} else {
		loginBtn.MustEval(`() => this.click()`)
	}

	page.MustWaitLoad()
	time.Sleep(5 * time.Second)
	page.MustScreenshot("debug_home.png")
	fmt.Println("  ✓ logged in")
	return nil
}

// findFirstElement tries a list of CSS selectors in order and returns the first match.
func findFirstElement(page *rod.Page, timeout time.Duration, selectors ...string) (*rod.Element, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, sel := range selectors {
			if el, err := page.Timeout(1 * time.Second).Element(sel); err == nil {
				return el, nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil, fmt.Errorf("none of %v found within %v", selectors, timeout)
}

func loadCookies(page *rod.Page) error {
	var data []byte
	if raw := os.Getenv("TWITTER_COOKIES"); raw != "" {
		data = []byte(raw)
	} else {
		var err error
		if data, err = os.ReadFile("cookies.json"); err != nil {
			return fmt.Errorf("cookies.json not found and TWITTER_COOKIES not set: %w", err)
		}
	}
	var cookies []Cookie
	if err := json.Unmarshal(data, &cookies); err != nil {
		return fmt.Errorf("parse cookies: %w", err)
	}
	for _, c := range cookies {
		if !strings.Contains(c.Domain, "twitter.com") && !strings.Contains(c.Domain, "x.com") {
			continue
		}
		cp := proto.NetworkCookieParam{
			Name: c.Name, Value: c.Value, Domain: c.Domain,
			Path: c.Path, HTTPOnly: c.HTTPOnly, Secure: c.Secure,
		}
		if c.Expires > 0 {
			cp.Expires = proto.TimeSinceEpoch(c.Expires)
		}
		if err := page.SetCookies([]*proto.NetworkCookieParam{&cp}); err != nil {
			return err
		}
	}
	return nil
}

func isErrorToast(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "already said") ||
		strings.Contains(lower, "something went wrong") ||
		strings.Contains(lower, "try again") ||
		strings.Contains(lower, "limit")
}

func toAbsURL(h string) string {
	if strings.HasPrefix(h, "/") {
		return "https://x.com" + h
	}
	return h
}

func mustElements(el interface{ Elements(string) (rod.Elements, error) }, sel string) rod.Elements {
	els, _ := el.Elements(sel)
	return els
}

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

// replyInSession is available for in-session replies without launching a new browser.
func (c *Client) replyInSession(page *rod.Page, article *rod.Element, comment string) error {
	replyBtn, err := article.Element(`[data-testid="reply"]`)
	if err != nil {
		return fmt.Errorf("reply button not found")
	}
	replyBtn.MustEval(`() => this.click()`)
	time.Sleep(2 * time.Second)

	replyBox, err := page.Timeout(sessionTimeout).Element(`[data-testid="tweetTextarea_0"]`)
	if err != nil {
		return fmt.Errorf("reply composer not found")
	}
	replyBox.MustEval(`() => this.focus()`)
	time.Sleep(300 * time.Millisecond)
	if err := page.InsertText(comment); err != nil {
		return fmt.Errorf("type: %w", err)
	}
	time.Sleep(800 * time.Millisecond)

	submitBtn, err := page.Timeout(sessionTimeout).Element(`[data-testid="tweetButton"]:not([disabled])`)
	if err != nil {
		page.Keyboard.Press(input.Escape) //nolint
		return fmt.Errorf("submit not found")
	}
	submitBtn.MustEval(`() => this.click()`)
	time.Sleep(3 * time.Second)
	return nil
}
