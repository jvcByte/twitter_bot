package content

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// creatorSystemPrompt is strict about no fabricated personal history.
const creatorSystemPrompt = `You are a Software Engineer and PCB Designer posting on X (Twitter).
Write short, direct, useful posts about software development, embedded systems, PCB design, and engineering.

STRICT RULES — never break these:
- NEVER claim specific years of experience ("5 years", "a decade", etc.)
- NEVER invent personal stories, past jobs, or specific projects you worked on
- NEVER say "I've been doing X for Y years" or "in my experience at [company]"
- DO share factual technical insights, opinions, and tips grounded in the source material
- DO write in plain direct language — opinions can be strong but must be factually grounded
- Add 1-2 relevant hashtags at the end (e.g. #EmbeddedSystems #PCBDesign #Golang #SoftwareEngineering #Firmware)
- ONLY tag someone if the post is directly about them or their work, using ONLY these verified handles:
  Dev: @dhh @unclebobmartin @martinfowler @kelseyhightower @b0rk @ThePrimeagen
  Embedded/Hardware: @adafruit @sparkfun @EEVblog @hackaday @bunniestudios
  Never invent or guess a Twitter handle.
- No quotes around the output. Just the raw tweet text.`

type creatorFormat struct {
	name   string
	prompt string // %s = article title, %s = source name, %s = article excerpt
}

var creatorFormats = []creatorFormat{
	{
		name: "code_tip",
		prompt: `Based on this article:
Title: %s
Source: %s
Excerpt: %s

Write a single practical coding tip tweet. Extract ONE specific, concrete technical insight.
Be specific — name the language, function, pattern, or tool. Avoid generic advice like "write clean code".
Example of BAD output: "TIL that writing tests saves time"
Example of GOOD output: "TIL Go's sync.Once is the cleanest way to init a singleton — no mutex needed 🔒"
Lead with the tip. No personal backstory.
Use emojis. Max 240 chars.`,
	},
	{
		name: "pcb_or_embedded_tip",
		prompt: `Based on this article:
Title: %s
Source: %s
Excerpt: %s

Write a single practical PCB or embedded systems tip tweet. Must be concrete and specific.
Name the specific component, protocol, IC, tool, or technique (e.g. "100nF decoupling cap", "UART idle line detection", "KiCad DRC").
Example of BAD output: "Good PCB layout matters"
Example of GOOD output: "Place your 100nF decoupling cap within 1mm of the VCC pin or it's basically useless ⚡"
No personal anecdotes.
Use emojis. Max 240 chars.`,
	},
	{
		name: "dev_opinion",
		prompt: `Based on this article:
Title: %s
Source: %s
Excerpt: %s

Write a strong opinion tweet reacting to the article. Must take a clear stance — agree, disagree, or challenge.
Be specific about what you're reacting to. No vague takes like "this is important".
Example of BAD output: "AI tools are changing development"
Example of GOOD output: "Hot take: GitHub Copilot is great for boilerplate but it makes junior devs skip understanding why code works 🤔"
Use emojis. Max 240 chars.`,
	},
	{
		name: "learning_moment",
		prompt: `Based on this article:
Title: %s
Source: %s
Excerpt: %s

Write a "TIL" or insight tweet grounded in the specific technical content of the article.
Must include the actual specific fact, not a vague summary.
Example of BAD output: "TIL that optimizing AI can reduce errors"
Example of GOOD output: "TIL that Go's escape analysis means stack-allocated variables are ~3x faster than heap — the compiler decides, not you 🚀"
No personal discovery story.
Use emojis. Max 240 chars.`,
	},
	{
		name: "tool_take",
		prompt: `Based on this article:
Title: %s
Source: %s
Excerpt: %s

Write an honest tweet about the specific tool, library, or technique discussed. Name it explicitly.
State one concrete thing that's good or bad about it with a reason.
Example of BAD output: "This tool is interesting"
Example of GOOD output: "Zephyr RTOS has the most complete device driver model for embedded — but the learning curve is steep if you're coming from bare metal 🔧"
Use emojis. Max 240 chars.`,
	},
	{
		name: "career_take",
		prompt: `Based on this article:
Title: %s
Source: %s
Excerpt: %s

Write a career or craft insight tweet for developers. Ground it in the article's specific content.
Must give concrete, actionable advice — not generic motivation.
Example of BAD output: "Keep learning and growing as a developer"
Example of GOOD output: "Reading other people's Git history teaches you more about architecture decisions than any tutorial 📖"
Use emojis. Max 240 chars.`,
	},
	{
		name: "honest_take",
		prompt: `Based on this article:
Title: %s
Source: %s
Excerpt: %s

Write a candid tweet about a real tradeoff or uncomfortable truth from the article's topic.
Must be specific — name the technology, pattern, or practice.
Example of BAD output: "Sometimes we skip best practices"
Example of GOOD output: "Real talk: most teams using microservices don't need them. They just made debugging 10x harder for a scalability problem they don't have yet 😬"
Use emojis. Max 240 chars.`,
	},
}

var creatorTextOnly = map[string]bool{
	"dev_opinion":  true,
	"career_take":  true,
	"honest_take":  true,
}

// GenerateCreatorPost fetches a real article from dev/embedded feeds and writes
// an owned-content tweet grounded in that article.
func GenerateCreatorPost(apiKey string) (string, string, error) {
	// Pull a real article from dev/embedded feeds
	seen := NewSeenStore("data/seen_creator.json")
	articles, err := Poll(seen, 48*time.Hour, "data/dev_feeds.json", "")
	if err != nil || len(articles) == 0 {
		return "", "", fmt.Errorf("no dev articles available: %v", err)
	}

	rand.Seed(time.Now().UnixNano())
	// Pick randomly from up to first 20 articles so we don't always use the newest
	cap := len(articles)
	if cap > 20 {
		cap = 20
	}
	a := articles[rand.Intn(cap)]

	// Fetch article text for richer context
	excerpt := fetchArticleText(a.Link)
	if len(excerpt) > 500 {
		excerpt = excerpt[:500]
	}
	if excerpt == "" {
		excerpt = a.Title
	}

	format := creatorFormats[rand.Intn(len(creatorFormats))]
	prompt := fmt.Sprintf(format.prompt, a.Title, a.FeedName, excerpt)

	post, err := callGroqWithSystem(apiKey, creatorSystemPrompt, prompt, 150)
	if err != nil {
		return "", "", err
	}
	post = strings.TrimSpace(strings.Trim(post, `"`))
	if len(post) > 280 {
		post = post[:277] + "..."
	}

	// Mark article as seen so we don't reuse it
	seen.Add(a.Link)

	return post, format.name, nil
}

// GenerateCreatorThread generates a factual thread based on a real dev article.
func GenerateCreatorThread(apiKey, topic string) ([]string, error) {
	seen := NewSeenStore("data/seen_creator.json")

	var a Article
	if topic == "" {
		articles, err := Poll(seen, 48*time.Hour, "data/dev_feeds.json", "")
		if err != nil || len(articles) == 0 {
			return nil, fmt.Errorf("no dev articles available: %v", err)
		}
		rand.Seed(time.Now().UnixNano())
		cap := len(articles)
		if cap > 20 {
			cap = 20
		}
		a = articles[rand.Intn(cap)]
		seen.Add(a.Link)
	}

	excerpt := fetchArticleText(a.Link)
	if len(excerpt) > 600 {
		excerpt = excerpt[:600]
	}
	if excerpt == "" {
		excerpt = a.Title
	}

	sourceInfo := ""
	if a.Title != "" {
		sourceInfo = fmt.Sprintf(`Source article: "%s" from %s
Excerpt: %s`, a.Title, a.FeedName, excerpt)
	} else {
		sourceInfo = fmt.Sprintf("Topic: %s", topic)
	}

	prompt := fmt.Sprintf(`Write a Twitter thread of 6 tweets for software engineers and PCB designers.
%s

Rules:
- Tweet 1: Hook — a bold, specific technical claim or insight from the source. End with "🧵"
- Tweets 2–5: Each delivers one concrete technical tip, tradeoff, or gotcha. Numbered (2/, 3/, etc.)
- Tweet 6: Strong closer — a practical takeaway or "save this" call to action.
- NEVER claim specific years of experience or invent personal project stories.
- Stay grounded in the source material — no invented facts.
- Each tweet <= 250 chars. Use emojis naturally. Add 1-2 relevant hashtags on the last tweet only.
- Output ONLY the 6 tweets, one per line, nothing else.`, sourceInfo)

	raw, err := callGroqWithSystem(apiKey, creatorSystemPrompt, prompt, 900)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(raw), "\n")
	var tweets []string
	for _, line := range lines {
		line = strings.TrimSpace(strings.Trim(line, `"`))
		if line == "" {
			continue
		}
		if len(line) > 280 {
			line = line[:277] + "..."
		}
		tweets = append(tweets, line)
	}
	if len(tweets) < 2 {
		return nil, fmt.Errorf("too few tweets generated: %d", len(tweets))
	}
	return tweets, nil
}

// callGroqWithSystem calls Groq with a custom system prompt.
func callGroqWithSystem(apiKey, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	reqBody := groqRequest{
		Model: "llama-3.3-70b-versatile",
		Messages: []groqMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens:   maxTokens,
		Temperature: 0.85,
	}
	return callGroqRaw(apiKey, reqBody)
}

// IsCreatorTextOnly returns true if the format performs better without an image.
func IsCreatorTextOnly(name string) bool {
	return creatorTextOnly[name]
}

// GenerateEngagementComment generates a short, genuine reply to someone else's tweet.
// The comment should add value — a follow-up question, a related insight, or agreement with context.
func GenerateEngagementComment(apiKey, tweetText string) (string, error) {
	prompt := fmt.Sprintf(`Someone posted this tweet:
"%s"

Write a SHORT reply (1-2 sentences) that adds genuine value to the conversation.
Rules:
- Add a related technical insight, ask a follow-up question, or share a brief relevant observation
- Sound like a real engineer — not a bot, not a marketer
- Do NOT just say "great post!" or "totally agree!" — be specific
- Do NOT start with "I" 
- Max 200 chars. No hashtags. Just the reply text.`, tweetText)

	reply, err := callGroqWithSystem(apiKey, creatorSystemPrompt, prompt, 80)
	if err != nil {
		return "", err
	}
	reply = strings.TrimSpace(strings.Trim(reply, `"`))
	if len(reply) > 280 {
		reply = reply[:277] + "..."
	}
	return reply, nil
}
