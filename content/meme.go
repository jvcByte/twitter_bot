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
	{
		name: "question",
		prompt: `Write a single open-ended question tweet for developers/tech people.
Should spark debate or personal reflection. Examples: "What's the one thing you wish you knew before your first dev job?",
"Which tech decision do you regret most?", "What's the most underrated skill in software engineering?"
Use emojis. Max 240 chars. No hashtags. Just the tweet text.`,
	},
	{
		name: "storytelling",
		prompt: `Write a single tweet that opens a relatable developer story or confession.
Format: start with "Story time:" or "True story:" or a hook like "I once spent 3 days debugging..."
Make it feel personal, honest, and funny. Use emojis. Max 240 chars. No hashtags. Just the tweet text.`,
	},
	{
		name: "educational",
		prompt: `Write a single punchy educational tweet for developers. Share one genuinely useful tip, trick,
or insight about programming, system design, career growth, or tools.
Format: lead with the insight, then a brief explanation. Use emojis. Max 240 chars. No hashtags. Just the tweet text.`,
	},
	{
		name: "news_reaction",
		prompt: `Given this tech headline: "%s"
Write a single tweet reacting to it with a strong opinion or take — agree, disagree, or add context.
Make it feel like a real person's genuine reaction, not a summary. Use emojis. Max 240 chars. No hashtags. Just the tweet text.`,
	},
}

// GenerateMemePost generates an AI-powered funny/engaging tweet using Groq.
// headline is optional — used for reaction/news_reaction formats. Pass empty string to skip.
func GenerateMemePost(apiKey, headline string) (string, error) {
	rand.Seed(time.Now().UnixNano())

	// Pick a random format; if no headline, skip formats that require one
	available := formats
	if headline == "" {
		filtered := formats[:0]
		for _, f := range formats {
			if f.name != "reaction" && f.name != "news_reaction" {
				filtered = append(filtered, f)
			}
		}
		available = filtered
	}
	format := available[rand.Intn(len(available))]

	prompt := format.prompt
	if (format.name == "reaction" || format.name == "news_reaction") && headline != "" {
		prompt = fmt.Sprintf(format.prompt, headline)
	}

	post, err := callGroq(apiKey, prompt, 120)
	if err != nil {
		return "", err
	}

	post = strings.TrimSpace(strings.Trim(post, `"`))
	if len(post) > 280 {
		post = post[:277] + "..."
	}
	return post, nil
}

// callGroq sends a user prompt to Groq and returns the raw text response.
func callGroq(apiKey, userPrompt string, maxTokens int) (string, error) {
	reqBody := groqRequest{
		Model: "llama-3.3-70b-versatile",
		Messages: []groqMessage{
			{
				Role: "system",
				Content: "You are a witty tech Twitter personality. You write short, punchy, " +
					"engaging tweets that get likes and retweets. Never use hashtags unless asked. " +
					"Never add explanations or quotes around the tweet. Just output the raw tweet text.",
			},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens:   maxTokens,
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
	return gr.Choices[0].Message.Content, nil
}

// GenerateThread generates a 5–8 tweet thread using Groq.
// topic is optional — if empty, Groq picks a relevant dev/tech topic.
// Returns a slice of tweet strings, each <= 280 chars.
func GenerateThread(apiKey, topic string) ([]string, error) {
	topicLine := "Pick an interesting software engineering or tech topic."
	if topic != "" {
		topicLine = fmt.Sprintf("Topic: %s", topic)
	}

	prompt := fmt.Sprintf(`Write a Twitter thread of 6 tweets for a developer audience.
%s

Rules:
- Tweet 1: Hook — a bold claim, surprising fact, or compelling question. End with "🧵"
- Tweets 2–5: Each tweet delivers one concrete insight, tip, or story beat. Numbered (2/, 3/, etc.)
- Tweet 6: Strong closer — a takeaway, call to action, or punchy conclusion.
- Each tweet must be <= 260 chars (leave room for numbering).
- Use emojis naturally. No hashtags.
- Output ONLY the 6 tweets, one per line, nothing else.`, topicLine)

	raw, err := callGroq(apiKey, prompt, 900)
	if err != nil {
		return nil, err
	}

	// Parse lines into individual tweets
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	var tweets []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.Trim(line, `"`)
		if line == "" {
			continue
		}
		if len(line) > 280 {
			line = line[:277] + "..."
		}
		tweets = append(tweets, line)
	}

	if len(tweets) < 2 {
		return nil, fmt.Errorf("groq returned too few thread tweets: %d", len(tweets))
	}
	return tweets, nil
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

// generateMemegenImage picks a random template and builds a memegen.link image URL,
// then downloads it to a temp file.
func generateMemegenImage(text0, text1 string) (string, error) {
	rand.Seed(time.Now().UnixNano())
	tmpl := memegenTemplates[rand.Intn(len(memegenTemplates))]

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
		return "", fmt.Errorf("memegen request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("memegen returned %d", resp.StatusCode)
	}

	f, err := os.CreateTemp("", "memegen_*.jpg")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("failed to write memegen image: %w", err)
	}
	return f.Name(), nil
}

type imgflipResponse struct {
	Success bool `json:"success"`
	Data    struct {
		URL string `json:"url"`
	} `json:"data"`
	ErrorMessage string `json:"error_message"`
}

// dev-culture meme templates on memegen.link
var memegenTemplates = []struct {
	id   string
	name string
}{
	{"drake", "Drake Hotline Bling"},
	{"db", "Distracted Boyfriend"},
	{"buttons", "Two Buttons"},
	{"brain", "Expanding Brain"},
	{"rollsafe", "Roll Safe"},
	{"oprah", "Oprah You Get"},
	{"buzz", "Buzz Lightyear Memes Everywhere"},
	{"doge", "Doge"},
	{"pigeon", "Is This a Pigeon"},
	{"ants", "Do You Want Ants"},
	{"afraid", "Afraid to Ask Andy"},
	{"fine", "This Is Fine"},
	{"fry", "Not Sure If"},
	{"iw", "Infinity War"},
	{"wonka", "Condescending Wonka"},
	{"ackbar", "It's A Trap"},
	{"success", "Success Kid"},
	{"yuno", "Y U No"},
	{"sparta", "This Is Sparta"},
	{"mordor", "One Does Not Simply"},
}

// memegenEncode encodes text for use in a memegen.link URL path segment.
func memegenEncode(s string) string {
	if len(s) > 80 {
		s = s[:80]
	}
	// Strip newlines, tabs, and any control characters
	var clean strings.Builder
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' || r < 32 {
			clean.WriteRune(' ')
		} else {
			clean.WriteRune(r)
		}
	}
	s = strings.TrimSpace(clean.String())

	r := strings.NewReplacer(
		" ", "_",
		"?", "~q",
		"&", "~a",
		"%", "~p",
		"#", "~h",
		"/", "~s",
		"\\", "~b",
		"<", "~l",
		">", "~g",
		`"`, "''",
	)
	return r.Replace(s)
}

// GenerateMemeImage creates a meme image using memegen.link (free, no auth required).
// Falls back to Imgflip if memegen fails and credentials are available.
// text0 = top text, text1 = bottom text.
func GenerateMemeImage(username, password, text0, text1 string) (string, error) {
	// Try memegen.link first — free, no credentials needed
	path, err := generateMemegenImage(text0, text1)
	if err != nil {
		fmt.Printf("  memegen failed: %v — falling back to imgflip\n", err)
	} else if path != "" {
		return path, nil
	}

	// Fallback: Imgflip
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
