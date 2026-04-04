package content

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqRequest struct {
	Model       string        `json:"model"`
	Messages    []groqMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature"`
}

type groqResponse struct {
	Choices []struct {
		Message groqMessage `json:"message"`
	} `json:"choices"`
}

// postFormat defines a tweet personality format
type postFormat struct {
	name   string
	prompt string
}

var formats = []postFormat{
	{
		name: "dev_humor",
		prompt: `Write a single funny tweet about software development or programming. 
Style: relatable dev humor like "it works on my machine", merge conflicts, CSS pain, 
Monday deploys, or debugging at 3am. Use emojis. Max 240 chars. No hashtags. Just the tweet text.`,
	},
	{
		name: "hot_take",
		prompt: `Write a single spicy tech hot take tweet starting with "Unpopular opinion:" or "Hot take:".
Make it about software development, programming languages, tools, or tech culture.
Should be slightly controversial but not offensive. Max 240 chars. No hashtags. Just the tweet text.`,
	},
	{
		name: "relatable",
		prompt: `Write a single relatable tweet for developers/tech people using the "me at X vs me at Y" format
or "nobody: / developers:" format. About coding, debugging, meetings, deadlines, or tech life.
Use emojis. Max 240 chars. No hashtags. Just the tweet text.`,
	},
	{
		name: "poll",
		prompt: `Write a single engaging Twitter poll question for developers. Format:
[Question]

🅰️ [Option A]
🅱️ [Option B]

Examples: tabs vs spaces, vim vs vscode, dark vs light mode, coffee vs tea while coding.
Keep it fun. Max 240 chars. Just the tweet text.`,
	},
	{
		name: "thread_starter",
		prompt: `Write a single tweet that starts a thread with "Things nobody tells you about [tech topic] 🧵"
or "X things I wish I knew before [tech thing]:". Make it feel like the start of a juicy thread.
Use emojis. Max 240 chars. No hashtags. Just the tweet text.`,
	},
	{
		name: "reaction",
		prompt: `Given this tech headline: "%s"
Write a single funny/witty tweet reacting to it from a developer's perspective.
Could be sarcastic, surprised, or humorous. Use emojis. Max 240 chars. No hashtags. Just the tweet text.`,
	},
}

// GenerateMemePost generates an AI-powered funny/engaging tweet using Groq.
// headline is optional — used for reaction format. Pass empty string to skip.
func GenerateMemePost(apiKey, headline string) (string, error) {
	rand.Seed(time.Now().UnixNano())

	// Pick a random format; if no headline, skip reaction format
	available := formats
	if headline == "" {
		available = formats[:5] // exclude reaction
	}
	format := available[rand.Intn(len(available))]

	prompt := format.prompt
	if format.name == "reaction" && headline != "" {
		prompt = fmt.Sprintf(format.prompt, headline)
	}

	reqBody := groqRequest{
		Model: "llama-3.3-70b-versatile",
		Messages: []groqMessage{
			{
				Role: "system",
				Content: "You are a witty tech Twitter personality. You write short, punchy, " +
					"engaging tweets that get likes and retweets. Never use hashtags unless asked. " +
					"Never add explanations or quotes around the tweet. Just output the raw tweet text.",
			},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   120,
		Temperature: 0.9,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions",
		bytes.NewBuffer(data))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("groq request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("groq API error (%d): %s", resp.StatusCode, string(body))
	}

	var gr groqResponse
	if err := json.Unmarshal(body, &gr); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}
	if len(gr.Choices) == 0 {
		return "", fmt.Errorf("no response from groq")
	}

	post := strings.TrimSpace(gr.Choices[0].Message.Content)

	// Strip surrounding quotes if the model added them
	post = strings.Trim(post, `"`)

	if len(post) > 280 {
		post = post[:277] + "..."
	}

	return post, nil
}
