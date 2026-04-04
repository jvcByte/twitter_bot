package content

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
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

// imgflip meme templates relevant to tech/dev culture
var memeTemplates = []struct {
	id   string
	name string
}{
	{"181913649", "Drake Hotline Bling"},
	{"87743020", "Two Buttons"},
	{"112126428", "Distracted Boyfriend"},
	{"131087935", "Running Away Balloon"},
	{"217743513", "UNO Draw 25 Cards"},
	{"124822590", "Left Exit 12 Off Ramp"},
	{"247375501", "Buff Doge vs. Cheems"},
	{"101470", "Ancient Aliens"},
	{"61579", "One Does Not Simply"},
	{"93895088", "Expanding Brain"},
	{"129242436", "Change My Mind"},
	{"148909805", "Monkey Puppet"},
	{"91538330", "X, X Everywhere"},
	{"4087833", "Waiting Skeleton"},
	{"135256802", "Epic Handshake"},
}

type imgflipResponse struct {
	Success bool `json:"success"`
	Data    struct {
		URL string `json:"url"`
	} `json:"data"`
	ErrorMessage string `json:"error_message"`
}

// GenerateMemeImage creates a meme image using Imgflip and returns a local temp file path.
// text0 = top text, text1 = bottom text.
// Returns ("", nil) if username/password are empty.
func GenerateMemeImage(username, password, text0, text1 string) (string, error) {
	if username == "" || password == "" {
		return "", nil
	}

	rand.Seed(time.Now().UnixNano())
	tmpl := memeTemplates[rand.Intn(len(memeTemplates))]

	resp, err := http.PostForm("https://api.imgflip.com/caption_image", map[string][]string{
		"template_id": {tmpl.id},
		"username":    {username},
		"password":    {password},
		"text0":       {text0},
		"text1":       {text1},
	})
	if err != nil {
		return "", fmt.Errorf("imgflip request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read imgflip response: %w", err)
	}

	var ir imgflipResponse
	if err := json.Unmarshal(body, &ir); err != nil {
		return "", fmt.Errorf("failed to parse imgflip response: %w", err)
	}
	if !ir.Success {
		return "", fmt.Errorf("imgflip error: %s", ir.ErrorMessage)
	}

	// Download the generated meme image
	imgResp, err := http.Get(ir.Data.URL)
	if err != nil {
		return "", fmt.Errorf("failed to download meme image: %w", err)
	}
	defer imgResp.Body.Close()

	f, err := os.CreateTemp("", "meme_*.jpg")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, imgResp.Body); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("failed to write meme image: %w", err)
	}

	fmt.Printf("  🖼  meme: %s (%s)\n", tmpl.name, ir.Data.URL)
	return f.Name(), nil
}
