// Package generation handles LLM-based content generation.
// Uses instruction-following models only — no reasoning/thinking models.
package generation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature"`
	Reasoning   *reasoningCfg `json:"reasoning,omitempty"`
}

type reasoningCfg struct {
	Enabled bool `json:"enabled"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type provider struct {
	name     string
	envKey   string
	url      string
	model    string
	fallback string
}

// providers is tried in order — instruction-following models only.
// Gemini 2.5 Flash Lite: 1500 req/day free (vs 20/day for 3.5-flash).
var providers = []provider{
	{
		name:     "OpenRouter-Gemma",
		envKey:   "OPENROUTER_API_KEY",
		url:      "https://openrouter.ai/api/v1/chat/completions",
		model:    "google/gemma-4-26b-a4b-it:free",
		fallback: "google/gemma-4-31b-it:free",
	},
	{
		name:     "OpenRouter-Nemotron",
		envKey:   "OPENROUTER_API_KEY",
		url:      "https://openrouter.ai/api/v1/chat/completions",
		model:    "nvidia/nemotron-3-ultra-550b-a55b:free",
		fallback: "nvidia/nemotron-3-super-120b-a12b:free",
	},
	{
		name:     "Gemini",
		envKey:   "GEMINI_API_KEY",
		url:      "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions",
		model:    "gemini-2.5-flash-lite",
		fallback: "gemini-2.5-flash",
	},
}

// activeProviders is set by ProbeProviders; nil means use full list.
var activeProviders []provider

var reasoningLeakPatterns = []string{
	"the user wants", "the user is asking", "i need to",
	"let me ", "i should ", "i'll write", "the tweet shows",
	"as an ai", "my task is",
}

func looksLikeReasoningLeak(s string) bool {
	lower := strings.ToLower(s)
	// Reasoning leaks are almost always at the very start of the response.
	head := lower
	if len(head) > 120 {
		head = head[:120]
	}
	for _, p := range reasoningLeakPatterns {
		if strings.Contains(head, p) {
			return true
		}
	}
	return false
}

// ProbeProviders tests each provider and keeps only the working ones.
func ProbeProviders() {
	req := chatRequest{
		Messages:    []chatMessage{{Role: "user", Content: "Say hello in one sentence."}},
		MaxTokens:   100,
		Temperature: 0,
		Reasoning:   &reasoningCfg{Enabled: false},
	}

	var working []provider
	for _, p := range providers {
		key := os.Getenv(p.envKey)
		if key == "" {
			continue
		}
		req.Model = p.model
		result, err := callPost(key, p.url, req)
		if err != nil && p.fallback != "" {
			req.Model = p.fallback
			result, err = callPost(key, p.url, req)
			if err == nil {
				p.model = p.fallback
				p.fallback = ""
			}
		}
		if err != nil {
			fmt.Printf("  ✗ %s: %v\n", p.name, err)
			continue
		}
		fmt.Printf("  ✓ %s (%s): %q\n", p.name, p.model, result)
		working = append(working, p)
	}

	if len(working) == 0 {
		fmt.Println("  ⚠ no LLM providers passed probe — will retry each call live")
		return
	}
	activeProviders = working
}

// knownHandles is the curated list of verified X accounts the LLM may tag.
const knownHandles = `AI/ML: @OpenAI @AnthropicAI @GoogleDeepMind @karpathy @ylecun @xAI @grok
Cybersecurity: @briankrebs @troyhunt @SwiftOnSecurity @thegrugq
Dev/Engineering: @dhh @unclebobmartin @martinfowler @b0rk @ThePrimeagen
PCB/Hardware: @adafruit @sparkfun @EEVblog @hackaday @bunniestudios
Tag @grok or @xAI when asking a question Grok could answer or reacting to xAI news.`

const defaultSystem = "You are a sharp, witty tech personality on X (Twitter) specialising in AI and cybersecurity. " +
	"Write short, punchy posts that get replies and retweets. " +
	"Tone: confident, relatable, occasionally provocative. " +
	"No markdown — no **bold**, no _italic_, no bullet points, no backticks. " +
	"No quotes around the output. Just raw tweet text. " +
	"Add 1-2 hashtags (e.g. #AI #CyberSecurity #DevLife). " +
	"Only tag accounts from this list when the post is directly about them: " + knownHandles +
	"Output ONLY the final tweet text and nothing else. " +
	"Do not describe the task, do not explain what you're about to do, " +
	"do not include phrases like 'the user wants' or 'here's a tweet'. " +
	"Your entire response must be the tweet itself, nothing before or after it. "

// Generate sends a prompt with the default system persona and returns the response.
func Generate(prompt string, maxTokens int) (string, error) {
	return GenerateWithSystem(defaultSystem, prompt, maxTokens)
}

// GenerateWithSystem sends a prompt with a custom system message.
func GenerateWithSystem(system, prompt string, maxTokens int) (string, error) {
	req := chatRequest{
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   maxTokens,
		Temperature: 0.85,
		Reasoning:   &reasoningCfg{Enabled: false},
	}

	pool := activeProviders
	if len(pool) == 0 {
		pool = providers
	}

	var lastErr error
	for _, p := range pool {
		key := os.Getenv(p.envKey)
		if key == "" {
			continue
		}
		req.Model = p.model
		result, err := callPost(key, p.url, req)
		if err != nil && strings.Contains(err.Error(), "404") && p.fallback != "" {
			req.Model = p.fallback
			result, err = callPost(key, p.url, req)
		}
		if err != nil {
			fmt.Printf("  ⚠ %s LLM failed: %v\n", p.name, err)
			lastErr = err
			continue
		}
		return result, nil
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("no LLM providers available — set GEMINI_API_KEY or OPENROUTER_API_KEY")
}

func callPost(apiKey, endpoint string, req chatRequest) (string, error) {
	// Gemini's OpenAI-compat endpoint does not support the "reasoning" field.
	if strings.Contains(endpoint, "generativelanguage.googleapis.com") {
		req.Reasoning = nil
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	do := func() (string, int, []byte, error) {
		r, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(body))
		if err != nil {
			return "", 0, nil, fmt.Errorf("build request: %w", err)
		}
		r.Header.Set("Authorization", "Bearer "+apiKey)
		r.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(r)
		if err != nil {
			return "", 0, nil, fmt.Errorf("request: %w", err)
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", resp.StatusCode, nil, fmt.Errorf("read: %w", err)
		}
		return "", resp.StatusCode, data, nil
	}

	_, status, data, err := do()
	if err != nil {
		return "", err
	}

	// On 429 retry once after the suggested delay (capped at 60s).
	if status == 429 {
		wait := 35 * time.Second
		// Try to parse retryDelay from the error body.
		if s := retryAfter(data); s > 0 && s <= 60 {
			wait = time.Duration(s) * time.Second
		}
		time.Sleep(wait)
		_, status, data, err = do()
		if err != nil {
			return "", err
		}
	}

	if status != 200 {
		return "", fmt.Errorf("API error (%d): %s", status, string(data))
	}

	var out chatResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("parse: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("empty response")
	}

	text := strings.TrimSpace(out.Choices[0].Message.Content)
	if text == "" {
		return "", fmt.Errorf("empty content in response")
	}
	text = StripMarkdown(trimQuotes(text))
	if len([]rune(text)) < 5 {
		return "", fmt.Errorf("response too short: %q", text)
	}
	if looksLikeReasoningLeak(text) {
		return "", fmt.Errorf("response looks like leaked reasoning, not a tweet: %q", text)
	}
	return text, nil
}

// retryAfter extracts a retry delay in seconds from a 429 response body.
func retryAfter(data []byte) int {
	s := string(data)
	// Look for patterns like "retry in 33s" or "retryDelay":"33s"
	for _, sep := range []string{`"retryDelay":"`, `retry in `, `Please retry in `} {
		if i := strings.Index(s, sep); i >= 0 {
			rest := s[i+len(sep):]
			var n int
			fmt.Sscanf(rest, "%d", &n)
			if n > 0 {
				return n
			}
		}
	}
	return 0
}

// TruncateTweet trims s to max runes at a word boundary.
func TruncateTweet(s string, max int) string {
	s = StripMarkdown(trimQuotes(strings.TrimSpace(s)))
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	cut := string(runes[:max-3])
	if idx := strings.LastIndex(cut, " "); idx > 0 {
		cut = cut[:idx]
	}
	return cut + "..."
}

// StripMarkdown removes markdown formatting that LLMs leak into output.
func StripMarkdown(s string) string {
	s = strings.NewReplacer("**", "", "__", "", "~~", "").Replace(s)
	words := strings.Fields(s)
	for i, w := range words {
		words[i] = strings.Trim(w, "*_`")
	}
	return strings.Join(words, " ")
}

func trimQuotes(s string) string {
	return strings.Trim(strings.TrimSpace(s), `"'`)
}
