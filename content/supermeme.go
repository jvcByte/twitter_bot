package content

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

const supermemeURL = "https://supermeme.ai/text-to-meme"
const supermemeTimeout = 45 * time.Second

type supermemeCookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expirationDate"`
	HTTPOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
	HostOnly bool    `json:"hostOnly"`
}

// GenerateSupermemeImage uses a headless browser to generate a contextually relevant
// meme image on supermeme.ai. Returns ("", nil) if SUPERMEME_COOKIES is not set.
func GenerateSupermemeImage(text string) (string, error) {
	cookieData := os.Getenv("SUPERMEME_COOKIES")
	if cookieData == "" {
		return "", nil
	}
	if len(text) > 2500 {
		text = text[:2500]
	}

	browser, page, err := launchSupermemeSession(cookieData)
	if err != nil {
		return "", fmt.Errorf("supermeme session failed: %w", err)
	}
	defer browser.MustClose()

	textarea, err := page.Timeout(supermemeTimeout).Element("textarea")
	if err != nil {
		page.MustScreenshot("debug_supermeme.png")
		return "", fmt.Errorf("supermeme textarea not found: %w", err)
	}
	textarea.MustEval(`() => this.focus()`)
	time.Sleep(300 * time.Millisecond)
	if err := page.InsertText(text); err != nil {
		return "", fmt.Errorf("supermeme: failed to type text: %w", err)
	}
	time.Sleep(500 * time.Millisecond)

	generateBtn, err := page.Timeout(supermemeTimeout).Element(`button[type="submit"][aria-label="Generate"]`)
	if err != nil {
		page.MustScreenshot("debug_supermeme.png")
		return "", fmt.Errorf("supermeme generate button not found: %w", err)
	}
	generateBtn.MustEval(`() => this.click()`)

	fmt.Println("  ⏳ waiting for supermeme results...")
	var imgURL string
	deadline := time.Now().Add(supermemeTimeout)
	attempt := 0
	for time.Now().Before(deadline) {
		time.Sleep(2 * time.Second)
		attempt++
		if attempt == 3 {
			page.MustScreenshot("debug_supermeme_results.png")
		}

		// Memes render on a <canvas> — export as PNG data URL
		result, err := page.Eval(`() => {
			const canvases = document.querySelectorAll('canvas');
			for (const c of canvases) {
				if (c.width > 100 && c.height > 100) {
					try { return c.toDataURL('image/png'); } catch(e) {}
				}
			}
			return '';
		}`)
		if err != nil || result.Value.String() == "" {
			continue
		}
		dataURL := result.Value.String()
		if strings.HasPrefix(dataURL, "data:image/") {
			imgURL = dataURL
			break
		}
	}

	if imgURL == "" {
		page.MustScreenshot("debug_supermeme.png")
		return "", fmt.Errorf("supermeme: no meme canvas found after timeout")
	}

	fmt.Println("  🖼  supermeme: canvas exported as PNG")
	return saveDataURL(imgURL)
}

func launchSupermemeSession(cookieJSON string) (*rod.Browser, *rod.Page, error) {
	l := launcher.New().
		Headless(true).
		Set("no-sandbox").
		Set("disable-setuid-sandbox").
		Set("disable-dev-shm-usage").
		Set("disable-gpu").
		Set("disable-blink-features", "AutomationControlled").
		Set("no-first-run").
		Set("user-agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36").
		Set("window-size", "1280,900")

	if path := supermemeChromium(); path != "" {
		l = l.Bin(path)
	} else if path, exists := launcher.LookPath(); exists {
		l = l.Bin(path)
	} else {
		return nil, nil, fmt.Errorf("no Chromium/Chrome binary found")
	}

	u, err := l.Launch()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	browser := rod.New().ControlURL(u).MustConnect()
	page := browser.MustPage("")
	page.MustEval(`() => Object.defineProperty(navigator, 'webdriver', { get: () => undefined })`)

	page.MustNavigate("https://supermeme.ai")
	page.MustWaitLoad()
	time.Sleep(2 * time.Second)

	if err := setSupermemeCookies(page, cookieJSON); err != nil {
		browser.MustClose()
		return nil, nil, fmt.Errorf("failed to set supermeme cookies: %w", err)
	}
	if err := injectSupermemeLocalStorage(page, cookieJSON); err != nil {
		browser.MustClose()
		return nil, nil, fmt.Errorf("failed to inject supermeme session: %w", err)
	}

	page.MustNavigate(supermemeURL)
	page.MustWaitLoad()
	time.Sleep(4 * time.Second)

	if _, err := page.Timeout(15 * time.Second).Element("textarea"); err != nil {
		page.MustScreenshot("debug_supermeme_auth.png")
		browser.MustClose()
		return nil, nil, fmt.Errorf("supermeme session invalid or expired — please refresh cookies")
	}
	fmt.Println("  [supermeme] session valid")
	return browser, page, nil
}

func setSupermemeCookies(page *rod.Page, raw string) error {
	var cookies []supermemeCookie
	if err := json.Unmarshal([]byte(raw), &cookies); err != nil {
		return fmt.Errorf("failed to parse supermeme cookies: %w", err)
	}
	for _, c := range cookies {
		if !strings.Contains(c.Domain, "supermeme.ai") {
			continue
		}
		domain := c.Domain
		if !strings.HasPrefix(domain, ".") && !c.HostOnly {
			domain = "." + domain
		}
		cookie := proto.NetworkCookieParam{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   domain,
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

func injectSupermemeLocalStorage(page *rod.Page, raw string) error {
	var cookies []supermemeCookie
	if err := json.Unmarshal([]byte(raw), &cookies); err != nil {
		return fmt.Errorf("failed to parse cookies: %w", err)
	}
	var baseKey, part0, part1 string
	for _, c := range cookies {
		if strings.HasSuffix(c.Name, "-auth-token.0") {
			part0 = c.Value
			baseKey = strings.TrimSuffix(c.Name, ".0")
		}
		if strings.HasSuffix(c.Name, "-auth-token.1") {
			part1 = c.Value
		}
	}
	if baseKey == "" || part0 == "" {
		return nil
	}
	combined := strings.TrimPrefix(part0+part1, "base64-")
	script := fmt.Sprintf(`() => { localStorage.setItem(%q, %q); }`, baseKey, combined)
	if _, err := page.Eval(script); err != nil {
		return fmt.Errorf("localStorage injection failed: %w", err)
	}
	fmt.Printf("  [supermeme] injected localStorage key: %s\n", baseKey)
	return nil
}

func saveDataURL(dataURL string) (string, error) {
	comma := strings.Index(dataURL, ",")
	if comma < 0 {
		return "", fmt.Errorf("invalid data URL format")
	}
	ext := ".png"
	if strings.Contains(dataURL[:comma], "jpeg") || strings.Contains(dataURL[:comma], "jpg") {
		ext = ".jpg"
	}
	decoded, err := base64Decode(dataURL[comma+1:])
	if err != nil {
		return "", fmt.Errorf("failed to decode canvas data URL: %w", err)
	}
	f, err := os.CreateTemp("", "supermeme_*"+ext)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(decoded); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("failed to write canvas image: %w", err)
	}
	return f.Name(), nil
}

func downloadMemeImage(imageURL string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("failed to download meme image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("meme image download returned %d", resp.StatusCode)
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
	f, err := os.CreateTemp("", "supermeme_*"+ext)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("failed to write meme image: %w", err)
	}
	return f.Name(), nil
}

func base64Decode(s string) ([]byte, error) {
	// Try standard then URL encoding
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		b, err = base64.URLEncoding.DecodeString(s)
	}
	if err != nil {
		// Try with padding stripped
		b, err = base64.RawStdEncoding.DecodeString(s)
	}
	return b, err
}

func supermemeChromium() string {
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
