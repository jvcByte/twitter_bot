// Package generation handles all LLM-based content generation via Groq.
package generation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

const groqModel = "llama-3.3-70b-versatile"
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
func callRaw(apiKey string, req groqRequest) (string, error) {
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
	return gr.Choices[0].Message.Content, nil
}

// TruncateTweet trims a string to fit within Twitter's 280 char limit.
func TruncateTweet(s string, max int) string {
	s = trimQuotes(s)
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}

func trimQuotes(s string) string {
	for len(s) > 0 && (s[0] == '"' || s[0] == ' ') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == '"' || s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	return s
}
