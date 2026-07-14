// Package images handles AI image generation and meme creation.
// Priority chain: Pollinations.ai → memegen.link → Imgflip
package images

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jvcByte/twitter_bot/internal/generation"
)

// GenerateForPost generates an image for a post using the full priority chain:
// Pollinations.ai (AI) → memegen.link → Imgflip
func GenerateForPost(groqAPIKey, imgflipUser, imgflipPass, text0, text1 string) (string, error) {
	fullText := text0
	if text1 != "" {
		fullText = text0 + " " + text1
	}

	// 1. Pollinations.ai — free, unlimited, AI-generated
	path, err := Pollinations(groqAPIKey, fullText)
	if err != nil {
		fmt.Printf("  pollinations failed: %v — falling back to memegen\n", err)
	} else if path != "" {
		return path, nil
	}

	// 2. memegen.link — free, template-based
	path, err = Memegen(text0, text1)
	if err != nil {
		fmt.Printf("  memegen failed: %v — falling back to imgflip\n", err)
	} else if path != "" {
		return path, nil
	}

	// 3. Imgflip — requires credentials
	if imgflipUser == "" || imgflipPass == "" {
		return "", nil
	}
	return Imgflip(imgflipUser, imgflipPass, text0, text1)
}

// Pollinations generates an image from a text prompt via Pollinations.ai.
func Pollinations(groqAPIKey, tweetText string) (string, error) {
	prompt := buildPrompt(groqAPIKey, tweetText)
	fmt.Printf("  🎨 image prompt: %s\n", prompt)
	return fetchPollinations(prompt)
}

func buildPrompt(groqAPIKey, tweetText string) string {
	if groqAPIKey == "" {
		return extractKeywords(tweetText)
	}
	q := fmt.Sprintf(`Convert this tweet into a concise Stable Diffusion image prompt (max 100 chars).
Tweet: "%s"

Rules:
- Focus on visual elements: objects, scenes, mood, style
- Add style keywords: "digital art", "cinematic", "photorealistic", "cyberpunk"
- For AI/tech: use "futuristic", "glowing circuits", "neural network", "holographic"
- For security: use "cybersecurity", "hacker", "dark web", "shield", "lock"
- No text, no words, no letters in the image
- Output ONLY the prompt.`, tweetText)

	result, err := generation.CallGroq(groqAPIKey, q, 80)
	if err != nil || strings.TrimSpace(result) == "" {
		return extractKeywords(tweetText)
	}
	return strings.TrimSpace(strings.Trim(result, `"`))
}

func extractKeywords(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.ContainsAny(lower, "hack breach malware"):
		return "cybersecurity hacker dark digital art cinematic"
	case strings.ContainsAny(lower, "ai machine learning model"):
		return "artificial intelligence neural network futuristic glowing circuits digital art"
	case strings.ContainsAny(lower, "security threat vulnerab"):
		return "cybersecurity shield lock protection digital art dark blue"
	case strings.ContainsAny(lower, "data privacy"):
		return "data privacy encryption secure digital art holographic"
	default:
		return "technology futuristic digital art cinematic lighting"
	}
}

func fetchPollinations(prompt string) (string, error) {
	encoded := url.PathEscape(prompt)
	imgURL := fmt.Sprintf(
		"https://image.pollinations.ai/prompt/%s?width=1024&height=576&nologo=true&enhance=true",
		encoded,
	)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(imgURL)
	if err != nil {
		return "", fmt.Errorf("pollinations request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("pollinations returned %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "image/") {
		return "", fmt.Errorf("pollinations non-image content-type: %s", ct)
	}

	ext := extFromCT(ct)
	f, err := os.CreateTemp("", "pollinations_*"+ext)
	if err != nil {
		return "", fmt.Errorf("temp file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("write: %w", err)
	}
	fmt.Printf("  🖼  pollinations: %s\n", imgURL)
	return f.Name(), nil
}

// Memegen generates a template meme via memegen.link.
func Memegen(text0, text1 string) (string, error) {
	combined := text0 + text1
	hash := 0
	for _, c := range combined {
		hash = hash*31 + int(c)
	}
	if hash < 0 {
		hash = -hash
	}
	tmpl := memegenTemplates[hash%len(memegenTemplates)]

	top := memegenEncode(text0)
	bot := memegenEncode(text1)
	if top == "" {
		top = "_"
	}
	if bot == "" {
		bot = "_"
	}

	imgURL := fmt.Sprintf("https://api.memegen.link/images/%s/%s/%s.jpg", tmpl.id, top, bot)
	fmt.Printf("  🖼  memegen: %s (%s)\n", tmpl.name, imgURL)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(imgURL)
	if err != nil {
		return "", fmt.Errorf("memegen request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("memegen returned %d", resp.StatusCode)
	}

	f, err := os.CreateTemp("", "memegen_*.jpg")
	if err != nil {
		return "", fmt.Errorf("temp file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("write: %w", err)
	}
	return f.Name(), nil
}

type imgflipResp struct {
	Success bool   `json:"success"`
	Data    struct{ URL string `json:"url"` } `json:"data"`
	ErrorMessage string `json:"error_message"`
}

// Imgflip generates a captioned meme via the Imgflip API.
func Imgflip(username, password, text0, text1 string) (string, error) {
	rand.Seed(time.Now().UnixNano())
	tmpl := imgflipTemplates[rand.Intn(len(imgflipTemplates))]

	resp, err := http.PostForm("https://api.imgflip.com/caption_image", map[string][]string{
		"template_id": {tmpl.id},
		"username":    {username},
		"password":    {password},
		"text0":       {text0},
		"text1":       {text1},
	})
	if err != nil {
		return "", fmt.Errorf("imgflip request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var ir imgflipResp
	if err := json.Unmarshal(body, &ir); err != nil {
		return "", fmt.Errorf("parse imgflip: %w", err)
	}
	if !ir.Success {
		return "", fmt.Errorf("imgflip: %s", ir.ErrorMessage)
	}

	imgResp, err := http.Get(ir.Data.URL)
	if err != nil {
		return "", fmt.Errorf("download meme: %w", err)
	}
	defer imgResp.Body.Close()

	f, err := os.CreateTemp("", "meme_*.jpg")
	if err != nil {
		return "", fmt.Errorf("temp file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, imgResp.Body); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("write: %w", err)
	}
	fmt.Printf("  🖼  meme: %s (%s)\n", tmpl.name, ir.Data.URL)
	return f.Name(), nil
}

func extFromCT(ct string) string {
	switch {
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "webp"):
		return ".webp"
	default:
		return ".jpg"
	}
}

func memegenEncode(s string) string {
	if len(s) > 80 {
		s = s[:80]
	}
	var clean strings.Builder
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' || r < 32 {
			clean.WriteRune(' ')
		} else {
			clean.WriteRune(r)
		}
	}
	s = strings.TrimSpace(clean.String())
	return strings.NewReplacer(
		" ", "_", "?", "~q", "&", "~a", "%", "~p",
		"#", "~h", "/", "~s", "\\", "~b",
		"<", "~l", ">", "~g", `"`, "''",
	).Replace(s)
}

var memegenTemplates = []struct{ id, name string }{
	{"drake", "Drake"}, {"db", "Distracted Boyfriend"}, {"buttons", "Two Buttons"},
	{"brain", "Expanding Brain"}, {"rollsafe", "Roll Safe"}, {"oprah", "Oprah"},
	{"buzz", "Buzz Everywhere"}, {"doge", "Doge"}, {"pigeon", "Is This a Pigeon"},
	{"ants", "Do You Want Ants"}, {"afraid", "Afraid to Ask Andy"}, {"fine", "This Is Fine"},
	{"fry", "Not Sure If"}, {"iw", "Infinity War"}, {"wonka", "Condescending Wonka"},
	{"ackbar", "It's A Trap"}, {"success", "Success Kid"}, {"yuno", "Y U No"},
	{"sparta", "This Is Sparta"}, {"mordor", "One Does Not Simply"},
}

var imgflipTemplates = []struct{ id, name string }{
	{"181913649", "Drake"}, {"87743020", "Two Buttons"}, {"112126428", "Distracted Boyfriend"},
	{"131087935", "Running Away Balloon"}, {"217743513", "UNO Draw 25"},
	{"124822590", "Left Exit 12"}, {"247375501", "Buff Doge vs Cheems"},
	{"93895088", "Expanding Brain"}, {"129242436", "Change My Mind"},
	{"135256802", "Epic Handshake"},
}
