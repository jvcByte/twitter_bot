// Package generation handles all LLM-based content generation via Groq.
package generation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// groqMessage is a single message in a Groq chat completion request.
type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// groqRequest is the Groq chat completion request body.
type groqRequest struct {
	Model       string        `json:"model"`
	Messages    []groqMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature"`
}

// groqResponse is the Groq chat completion response body.
type groqResponse struct {
	Choices []struct {
		Message groqMessage `json:"message"`
	} `json:"choices"`
}

const groqModel = "openai/gpt-oss-20b"   // primary — no chain-of-thought issues
const groqModelFallback = "qwen/qwen3.6-27b" // fallback
const groqEndpoint = "https://api.groq.com/openai/v1/chat/completions"

// knownHandles is a curated list of verified accounts the LLM may tag.
// Never invent handles outside this list.
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
	"NEVER use markdown formatting — no **bold**, no _italic_, no bullet points, no headers. Plain text only. " +
	"Never add explanations or quotes around the tweet. " +
	"Add 1-2 relevant hashtags at the end (e.g. #AI #CyberSecurity #DevLife #Coding). " +
	"ONLY tag someone if the post is directly about them or their work, and ONLY use handles from this verified list: " + knownHandles +
	" Never invent or guess handles. Just output the raw tweet text."

// CallGroq calls Groq with the default AI/security system prompt.
func CallGroq(apiKey, userPrompt string, maxTokens int) (string, error) {
	return CallGroqWithSystem(apiKey, defaultSystemPrompt, userPrompt, maxTokens)
}

// CallGroqWithSystem calls Groq with a custom system prompt.
func CallGroqWithSystem(apiKey, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	return callRaw(apiKey, groqRequest{
		Model: groqModel,
		Messages: []groqMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens:   maxTokens,
		Temperature: 0.85,
	})
}

// callRaw executes a groqRequest and returns the first choice's content.
// On model-not-found (404), retries once with the fallback model.
func callRaw(apiKey string, req groqRequest) (string, error) {
	result, err := callRawOnce(apiKey, req)
	if err != nil && strings.Contains(err.Error(), "model_not_found") && req.Model != groqModelFallback {
		req.Model = groqModelFallback
		return callRawOnce(apiKey, req)
	}
	return result, err
}

func callRawOnce(apiKey string, req groqRequest) (string, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	httpReq, err := http.NewRequest("POST", groqEndpoint, bytes.NewBuffer(data))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("groq request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("groq API error (%d): %s", resp.StatusCode, string(body))
	}

	var gr groqResponse
	if err := json.Unmarshal(body, &gr); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if len(gr.Choices) == 0 {
		return "", fmt.Errorf("empty response from groq")
	}
	raw := gr.Choices[0].Message.Content
	result := stripThinking(raw)
	if result == "" && raw != "" {
		// stripThinking removed everything — return raw content as fallback
		// (better to have a thinking-prefixed reply than nothing)
		result = strings.TrimSpace(raw)
	}
	return result, nil
}

// TruncateTweet strips markdown artifacts and trims to max runes (not bytes).
func TruncateTweet(s string, max int) string {
	s = StripMarkdown(trimQuotes(s))
	runes := []rune(s)
	if len(runes) > max {
		return string(runes[:max-3]) + "..."
	}
	return s
}

// stripThinking removes Qwen3 <think>...</think> reasoning blocks from output.
// Handles all cases: closed blocks, unclosed blocks, and answer inside thinking.
func stripThinking(s string) string {
	// Case 1: proper <think>...</think> — strip the block, keep what follows
	result := s
	for {
		start := strings.Index(result, "<think>")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "</think>")
		if end == -1 {
			// Case 2: unclosed <think> — the "answer" is the last non-empty
			// line(s) inside the block (Qwen puts answer at end of thinking)
			thinking := result[start+len("<think>"):]
			result = extractLastLines(thinking)
			return strings.TrimSpace(result)
		}
		absEnd := start + end + len("</think>")
		result = result[:start] + result[absEnd:]
	}
	result = strings.TrimSpace(result)
	return result
}

// extractLastLines returns the last 1-3 meaningful lines of a string —
// used to pull the actual answer out of an unclosed think block.
func extractLastLines(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	var meaningful []string
	for i := len(lines) - 1; i >= 0 && len(meaningful) < 3; i-- {
		line := strings.TrimSpace(lines[i])
		// Skip reasoning meta-lines that start with numbered steps or bullets
		if line == "" || strings.HasPrefix(line, "1.") || strings.HasPrefix(line, "2.") ||
			strings.HasPrefix(line, "3.") || strings.HasPrefix(line, "-") ||
			strings.HasPrefix(line, "*") || strings.HasPrefix(line, "Here") ||
			strings.HasPrefix(line, "Analyze") || strings.HasPrefix(line, "The") {
			continue
		}
		meaningful = append([]string{line}, meaningful...)
	}
	if len(meaningful) == 0 {
		// All lines looked like reasoning — just take the last non-empty line
		for i := len(lines) - 1; i >= 0; i-- {
			if line := strings.TrimSpace(lines[i]); line != "" {
				return line
			}
		}
	}
	return strings.Join(meaningful, " ")
}

// StripMarkdown removes common markdown formatting that LLMs leak into output.
func StripMarkdown(s string) string {
	// Remove bold/italic markers: ** __ * _
	replacer := strings.NewReplacer(
		"**", "",
		"__", "",
		"~~", "",
	)
	s = replacer.Replace(s)
	// Remove single * and _ only when used as emphasis (not in usernames/emojis)
	// Simple approach: strip leading/trailing * or _ from each word
	words := strings.Fields(s)
	for i, w := range words {
		w = strings.TrimLeft(w, "*_`")
		w = strings.TrimRight(w, "*_`")
		words[i] = w
	}
	return strings.Join(words, " ")
}

func trimQuotes(s string) string {
	s = strings.TrimSpace(s)
	for len(s) > 0 && (s[0] == '"' || s[0] == '\'') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == '"' || s[len(s)-1] == '\'') {
		s = s[:len(s)-1]
	}
	return strings.TrimSpace(s)
}
