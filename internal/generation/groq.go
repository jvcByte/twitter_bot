// Package generation handles all LLM-based content generation.
// Provider priority: Cerebras → Gemini → OpenRouter → Groq
// All providers use the OpenAI-compatible chat completions format.
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

// groqMessage is a single message in a chat completion request.
type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// groqRequest is the chat completion request body (OpenAI-compatible).
type groqRequest struct {
	Model       string        `json:"model"`
	Messages    []groqMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature"`
}

// groqResponse is the chat completion response body.
type groqResponse struct {
	Choices []struct {
		Message groqMessage `json:"message"`
	} `json:"choices"`
}

// llmProvider defines an OpenAI-compatible LLM endpoint.
type llmProvider struct {
	name   string
	envKey string
	url    string
	model  string
}

// providers are tried in order until one succeeds.
var providers = []llmProvider{
	{
		name:   "Cerebras",
		envKey: "CEREBRAS_API_KEY",
		url:    "https://api.cerebras.ai/v1/chat/completions",
		model:  "gpt-oss-120b",
	},
	{
		name:   "Gemini",
		envKey: "GEMINI_API_KEY",
		url:    "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions",
		model:  "gemini-3.6-flash",
	},
	{
		name:   "OpenRouter",
		envKey: "OPENROUTER_API_KEY",
		url:    "https://openrouter.ai/api/v1/chat/completions",
		model:  "deepseek/deepseek-v3-0324:free",
	},
	{
		name:   "Groq",
		envKey: "GROQ_API_KEY",
		url:    "https://api.groq.com/openai/v1/chat/completions",
		model:  "openai/gpt-oss-20b",
	},
}

// knownHandles is a curated list of verified accounts the LLM may tag.
const knownHandles = `
AI/ML: @OpenAI @AnthropicAI @GoogleDeepMind @sama @karpathy @ylecun @GaryMarcus @emollick @swyx @goodside @xAI @grok
Cybersecurity: @briankrebs @schneierblog @threatpost @DarkReading @troyhunt @SwiftOnSecurity @thegrugq @taviso
Dev/Engineering: @dhh @unclebobmartin @martinfowler @kelseyhightower @jessfraz @masnick @b0rk @ThePrimeagen
PCB/Embedded/Hardware: @adafruit @sparkfun @EEVblog @hackaday @jeri_ellsworth @bunniestudios
Tech companies: @github @vercel @cloudflare @hashicorp @dockerhub

Tag @grok or @xAI when asking a question that Grok could answer, debating an AI topic, or reacting to xAI news.
Example: "Is AI actually making developers less skilled? 🤔 @grok what do you think? #AI #DevLife"
`

// defaultSystemPrompt is the persona for AI/security posts.
const defaultSystemPrompt = "You are a sharp, witty tech personality on X (Twitter) who specializes in AI and cybersecurity. " +
	"You write short, punchy, engaging posts that get replies, likes, and retweets. " +
	"Your tone is confident, relatable, and occasionally provocative — like a developer who's seen it all. " +
	"You favor AI tools, security threats, coding culture, and tech career topics. " +
	"NEVER use markdown formatting — no **bold**, no _italic_, no bullet points, no backticks as emphasis. " +
	"Never add explanations or quotes around the tweet. " +
	"Add 1-2 relevant hashtags at the end (e.g. #AI #CyberSecurity #DevLife #Coding). " +
	"ONLY tag someone if the post is directly about them or their work, and ONLY use handles from this verified list: " + knownHandles +
	" Never invent or guess handles. Just output the raw tweet text."

// CallGroq calls with the default AI/security system prompt.
// Name kept for backward compatibility.
func CallGroq(apiKey, userPrompt string, maxTokens int) (string, error) {
	return CallGroqWithSystem(apiKey, defaultSystemPrompt, userPrompt, maxTokens)
}

// CallGroqWithSystem calls with a custom system prompt, trying all providers in order.
// apiKey is accepted for backward compat but providers read keys from env directly.
func CallGroqWithSystem(_, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	req := groqRequest{
		Messages: []groqMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens:   maxTokens,
		Temperature: 0.85,
	}

	var lastErr error
	for _, p := range providers {
		key := os.Getenv(p.envKey)
		if key == "" {
			continue
		}
		req.Model = p.model
		result, err := callEndpoint(key, p.url, req)
		if err != nil {
			fmt.Printf("  ⚠ %s LLM failed: %v\n", p.name, err)
			lastErr = err
			continue
		}
		return stripThinking(result), nil
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("no LLM API keys configured (set CEREBRAS_API_KEY, GEMINI_API_KEY, OPENROUTER_API_KEY, or GROQ_API_KEY)")
}

// callEndpoint sends a request to any OpenAI-compatible endpoint.
func callEndpoint(apiKey, endpointURL string, req groqRequest) (string, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	httpReq, err := http.NewRequest("POST", endpointURL, bytes.NewBuffer(data))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var gr groqResponse
	if err := json.Unmarshal(body, &gr); err != nil {
		return "", fmt.Errorf("parse: %w", err)
	}
	if len(gr.Choices) == 0 {
		return "", fmt.Errorf("empty response")
	}

	raw := gr.Choices[0].Message.Content
	result := stripThinking(raw)
	if result == "" && raw != "" {
		result = strings.TrimSpace(raw)
	}
	return result, nil
}

// TruncateTweet strips markdown artifacts and trims to max runes (not bytes).
func TruncateTweet(s string, max int) string {
	s = StripMarkdown(stripThinking(trimQuotes(s)))
	runes := []rune(s)
	if len(runes) > max {
		return string(runes[:max-3]) + "..."
	}
	return s
}

// StripMarkdown removes common markdown formatting that LLMs leak into output.
func StripMarkdown(s string) string {
	replacer := strings.NewReplacer("**", "", "__", "", "~~", "")
	s = replacer.Replace(s)
	words := strings.Fields(s)
	for i, w := range words {
		w = strings.TrimLeft(w, "*_`")
		w = strings.TrimRight(w, "*_`")
		words[i] = w
	}
	return strings.Join(words, " ")
}

// stripThinking removes chain-of-thought <think>...</think> blocks.
// Handles closed blocks, unclosed blocks, and answer-inside-thinking.
func stripThinking(s string) string {
	result := s
	for {
		start := strings.Index(result, "<think>")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "</think>")
		if end == -1 {
			// Unclosed — extract the last meaningful line from inside the block
			thinking := result[start+len("<think>"):]
			return extractLastMeaningfulLine(thinking)
		}
		absEnd := start + end + len("</think>")
		result = result[:start] + result[absEnd:]
	}
	return strings.TrimSpace(result)
}

// extractLastMeaningfulLine pulls the last non-reasoning line from thinking content.
func extractLastMeaningfulLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	// Walk backwards, skip obvious reasoning meta-lines
	skip := []string{"1.", "2.", "3.", "4.", "5.", "6.", "7.", "8.", "9.",
		"- ", "* ", "Here", "Analyze", "Think", "Let me", "I need", "I should",
		"Step", "First", "Next", "Finally", "The tweet", "My role", "Source"}
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		isReasoning := false
		for _, prefix := range skip {
			if strings.HasPrefix(line, prefix) {
				isReasoning = true
				break
			}
		}
		if !isReasoning {
			return line
		}
	}
	// Everything looked like reasoning — return last non-empty line
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			return line
		}
	}
	return strings.TrimSpace(s)
}

func trimQuotes(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"'`)
	return strings.TrimSpace(s)
}
