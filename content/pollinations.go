package content

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// GeneratePollinationsImage generates a contextually relevant AI image from a tweet text.
// It first uses Groq to craft a good image prompt, then hits Pollinations.ai to generate it.
// Returns a local temp file path. No API key or credits required.
func GeneratePollinationsImage(groqAPIKey, tweetText string) (string, error) {
	// Step 1: Use Groq to generate a good image prompt from the tweet text
	imagePrompt := buildImagePrompt(groqAPIKey, tweetText)
	fmt.Printf("  🎨 image prompt: %s\n", imagePrompt)

	// Step 2: Hit Pollinations.ai — free, no auth, just a URL
	return fetchPollinationsImage(imagePrompt)
}

// buildImagePrompt uses Groq to turn tweet text into a good Stable Diffusion prompt.
// Falls back to a simple keyword extraction if Groq is unavailable.
func buildImagePrompt(groqAPIKey, tweetText string) string {
	if groqAPIKey == "" {
		return extractKeywords(tweetText)
	}

	prompt := fmt.Sprintf(`Convert this tweet into a concise Stable Diffusion image prompt (max 100 chars).
Tweet: "%s"

Rules:
- Focus on visual elements: objects, scenes, mood, style
- Add style keywords: "digital art", "cinematic", "photorealistic", "cyberpunk" etc.
- For AI/tech topics: use "futuristic", "glowing circuits", "neural network", "holographic"
- For security topics: use "cybersecurity", "hacker", "dark web", "shield", "lock"
- No text, no words, no letters in the image
- Output ONLY the prompt, nothing else.`, tweetText)

	result, err := callGroq(groqAPIKey, prompt, 80)
	if err != nil || strings.TrimSpace(result) == "" {
		return extractKeywords(tweetText)
	}
	return strings.TrimSpace(strings.Trim(result, `"`))
}

// extractKeywords does a simple keyword extraction as fallback when Groq is unavailable.
func extractKeywords(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "hack") || strings.Contains(lower, "breach") || strings.Contains(lower, "malware"):
		return "cybersecurity hacker dark digital art cinematic"
	case strings.Contains(lower, "ai") || strings.Contains(lower, "machine learning") || strings.Contains(lower, "model"):
		return "artificial intelligence neural network futuristic glowing circuits digital art"
	case strings.Contains(lower, "security") || strings.Contains(lower, "threat") || strings.Contains(lower, "vulnerab"):
		return "cybersecurity shield lock protection digital art dark blue"
	case strings.Contains(lower, "data") || strings.Contains(lower, "privacy"):
		return "data privacy encryption secure digital art holographic"
	default:
		return "technology futuristic digital art cinematic lighting"
	}
}

// fetchPollinationsImage downloads an AI-generated image from Pollinations.ai.
func fetchPollinationsImage(imagePrompt string) (string, error) {
	// Encode the prompt for URL
	encoded := url.PathEscape(imagePrompt)
	imgURL := fmt.Sprintf(
		"https://image.pollinations.ai/prompt/%s?width=1024&height=576&nologo=true&enhance=true",
		encoded,
	)

	client := &http.Client{Timeout: 60 * time.Second} // Pollinations can be slow
	resp, err := client.Get(imgURL)
	if err != nil {
		return "", fmt.Errorf("pollinations request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("pollinations returned %d", resp.StatusCode)
	}

	// Verify it's actually an image
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "image/") {
		return "", fmt.Errorf("pollinations returned non-image content-type: %s", ct)
	}

	ext := ".jpg"
	switch {
	case strings.Contains(ct, "png"):
		ext = ".png"
	case strings.Contains(ct, "webp"):
		ext = ".webp"
	}

	f, err := os.CreateTemp("", "pollinations_*"+ext)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("failed to write image: %w", err)
	}

	fmt.Printf("  🖼  pollinations: %s\n", imgURL)
	return f.Name(), nil
}
