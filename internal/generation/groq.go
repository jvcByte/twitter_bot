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
	name      string
	envKey    string
	url       string
	model     string
	fallback  string // alternate model to try on 404
}

// providers are tried in order until one succeeds.
// Priority: Groq → Gemini → OpenRouter → Cerebras
// Model IDs verified against each provider's live /models endpoint.
var providers = []llmProvider{
	{
		name:     "Groq",
		envKey:   "GROQ_API_KEY",
		url:      "https://api.groq.com/openai/v1/chat/completions",
		model:    "openai/gpt-oss-20b",
		fallback: "qwen/qwen3.6-27b",
	},
	{
		name:     "Gemini",
		envKey:   "GEMINI_API_KEY",
		url:      "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions",
		model:    "models/gemini-3.7-flash",
		fallback: "models/gemini-3.5-flash",
	},
	{
		name:     "OpenRouter",
		envKey:   "OPENROUTER_API_KEY",
		url:      "https://openrouter.ai/api/v1/chat/completions",
		model:    "openai/gpt-oss-20b:free",
		fallback: "nvidia/nemotron-3-super-120b-a12b:free",
	},
	{
		name:     "Cerebras",
		envKey:   "CEREBRAS_API_KEY",
		url:      "https://api.cerebras.ai/v1/chat/completions",
		model:    "gpt-oss-120b",
		fallback: "gemma-4-31b",
	},
}

// activeProviders is set once by ProbeProviders and used for all subsequent calls.
// nil means not probed yet — fall back to full providers list.
var activeProviders []llmProvider

// ProbeProviders tests each configured provider with a minimal prompt and
// removes any that fail. Call once at startup before posting.
func ProbeProviders() {
	probe := groqRequest{
		Messages: []groqMessage{
			{Role: "user", Content: "Reply with exactly: ok"},
		},
		MaxTokens:   20,
		Temperature: 0,
	}

	var working []llmProvider
	for _, p := range providers {
		key := os.Getenv(p.envKey)
		if key == "" {
			continue
		}
		probe.Model = p.model
		result, err := callEndpoint(key, p.url, probe)
		if err != nil {
			// Try fallback model
			if p.fallback != "" {
				probe.Model = p.fallback
				result, err = callEndpoint(key, p.url, probe)
				if err == nil {
					p.model = p.fallback // promote fallback to primary
					p.fallback = ""
				}
			}
		}
		if err != nil {
			fmt.Printf("  ✗ %s: %v\n", p.name, err)
			continue
		}
		fmt.Printf("  ✓ %s (%s): %q\n", p.name, p.model, strings.TrimSpace(result))
		working = append(working, p)
	}

	if len(working) == 0 {
		fmt.Println("  ⚠ no LLM providers passed probe — will retry each call live")
		activeProviders = nil
		return
	}
	activeProviders = working
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
	// Append explicit instruction to output answer after any thinking
	augmentedPrompt := userPrompt + "\n\nIMPORTANT: Output ONLY the final answer as plain text. No reasoning, no labels, no prefixes."
	req := groqRequest{
		Messages: []groqMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: augmentedPrompt},
		},
		MaxTokens:   maxTokens,
		Temperature: 0.85,
	}

	var lastErr error
	pool := activeProviders
	if pool == nil {
		pool = providers // not probed yet, try all
	}
	for _, p := range pool {
		key := os.Getenv(p.envKey)
		if key == "" {
			continue
		}
		req.Model = p.model
		result, err := callEndpoint(key, p.url, req)
		if err != nil {
			// On 404 try the fallback model before giving up on this provider
			if strings.Contains(err.Error(), "404") && p.fallback != "" {
				req.Model = p.fallback
				result, err = callEndpoint(key, p.url, req)
			}
			if err != nil {
				fmt.Printf("  ⚠ %s LLM failed: %v\n", p.name, err)
				lastErr = err
				continue
			}
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
	// Reject suspiciously short outputs — likely a failed/empty response
	if len([]rune(result)) < 5 {
		return "", fmt.Errorf("response too short (%d chars): %q", len(result), result)
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
func stripThinking(s string) string {
	result := s
	for {
		start := strings.Index(result, "<think>")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "</think>")
		if end == -1 {
			// Unclosed block — everything from here is reasoning, discard it
			result = strings.TrimSpace(result[:start])
			break
		}
		absEnd := start + end + len("</think>")
		result = result[:start] + result[absEnd:]
	}
	return strings.TrimSpace(result)
}

func trimQuotes(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"'`)
	return strings.TrimSpace(s)
}
