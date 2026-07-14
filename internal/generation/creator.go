package generation

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/jvcByte/twitter_bot/internal/feeds"
)

const creatorSystemPrompt = `You are a Software Engineer and PCB Designer posting on X (Twitter).
Write short, direct, useful posts about software development, embedded systems, PCB design, and engineering.

STRICT RULES — never break these:
- NEVER claim specific years of experience ("5 years", "a decade", etc.)
- NEVER invent personal stories, past jobs, or specific projects you worked on
- DO share factual technical insights grounded in the source material
- Be specific — name tools, languages, components, protocols
- Add 1-2 relevant hashtags (e.g. #EmbeddedSystems #PCBDesign #Golang #SoftwareEngineering)
- ONLY tag from these verified handles:
  Dev: @dhh @unclebobmartin @martinfowler @kelseyhightower @b0rk @ThePrimeagen
  Embedded/Hardware: @adafruit @sparkfun @EEVblog @hackaday @bunniestudios
- No quotes around the output. Just the raw tweet text.`

// CreatorTextOnly formats that perform better without images.
var CreatorTextOnly = map[string]bool{
	"dev_opinion":  true,
	"career_take":  true,
	"honest_take":  true,
}

type creatorFormat struct {
	name   string
	prompt string
}

var creatorFormats = []creatorFormat{
	{name: "code_tip", prompt: `Based on this article:
Title: %s | Source: %s
Excerpt: %s

Write a single practical coding tip tweet. Extract ONE specific, concrete technical insight.
Name the language, function, pattern, or tool explicitly.
BAD: "TIL that writing tests saves time"
GOOD: "TIL Go's sync.Once is the cleanest way to init a singleton — no mutex needed 🔒"
Lead with the tip. Max 240 chars.`},
	{name: "pcb_or_embedded_tip", prompt: `Based on this article:
Title: %s | Source: %s
Excerpt: %s

Write a single practical PCB or embedded systems tip. Must be concrete and specific.
Name the component, protocol, IC, tool, or technique.
BAD: "Good PCB layout matters"
GOOD: "Place your 100nF decoupling cap within 1mm of the VCC pin or it's basically useless ⚡"
Max 240 chars.`},
	{name: "dev_opinion", prompt: `Based on this article:
Title: %s | Source: %s
Excerpt: %s

Write a strong opinion tweet reacting to the article. Take a clear stance.
BAD: "AI tools are changing development"
GOOD: "Hot take: GitHub Copilot is great for boilerplate but makes junior devs skip understanding why code works 🤔"
Max 240 chars.`},
	{name: "learning_moment", prompt: `Based on this article:
Title: %s | Source: %s
Excerpt: %s

Write a "TIL" tweet with the actual specific technical fact from the article.
BAD: "TIL that optimizing AI can reduce errors"
GOOD: "TIL Go's escape analysis means stack-allocated vars are ~3x faster than heap — compiler decides, not you 🚀"
Max 240 chars.`},
	{name: "tool_take", prompt: `Based on this article:
Title: %s | Source: %s
Excerpt: %s

Write an honest tweet about the specific tool discussed. Name it. State one concrete good or bad thing.
BAD: "This tool is interesting"
GOOD: "Zephyr RTOS has the most complete device driver model for embedded — but steep learning curve from bare metal 🔧"
Max 240 chars.`},
	{name: "career_take", prompt: `Based on this article:
Title: %s | Source: %s
Excerpt: %s

Write a career insight tweet for developers. Ground it in the article. Give concrete advice.
BAD: "Keep learning and growing"
GOOD: "Reading other people's Git history teaches you more about architecture decisions than any tutorial 📖"
Max 240 chars.`},
	{name: "honest_take", prompt: `Based on this article:
Title: %s | Source: %s
Excerpt: %s

Write a candid tweet about a real tradeoff or uncomfortable truth. Name the technology.
BAD: "Sometimes we skip best practices"
GOOD: "Real talk: most teams using microservices don't need them. Just made debugging 10x harder 😬"
Max 240 chars.`},
}

// GenerateCreatorPost fetches a real article from dev/embedded feeds and generates
// an owned-content tweet grounded in it.
func GenerateCreatorPost(apiKey string) (string, string, error) {
	seen := feeds.NewSeenStore("data/seen_creator.json")
	articles, err := feeds.Poll(seen, 48*time.Hour, "data/dev_feeds.json", "")
	if err != nil || len(articles) == 0 {
		return "", "", fmt.Errorf("no dev articles available: %v", err)
	}

	rand.Seed(time.Now().UnixNano())
	cap := len(articles)
	if cap > 20 {
		cap = 20
	}
	a := articles[rand.Intn(cap)]

	excerpt := feeds.FetchText(a.Link)
	if len(excerpt) > 500 {
		excerpt = excerpt[:500]
	}
	if excerpt == "" {
		excerpt = a.Title
	}

	format := creatorFormats[rand.Intn(len(creatorFormats))]
	prompt := fmt.Sprintf(format.prompt, a.Title, a.FeedName, excerpt)

	post, err := CallGroqWithSystem(apiKey, creatorSystemPrompt, prompt, 150)
	if err != nil {
		return "", "", err
	}
	seen.Add(a.Link)
	return TruncateTweet(post, 280), format.name, nil
}

// GenerateCreatorThread generates a 6-tweet technical thread from a real dev article.
func GenerateCreatorThread(apiKey, topic string) ([]string, error) {
	seen := feeds.NewSeenStore("data/seen_creator.json")

	var a feeds.Article
	if topic == "" {
		articles, err := feeds.Poll(seen, 48*time.Hour, "data/dev_feeds.json", "")
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

	excerpt := feeds.FetchText(a.Link)
	if len(excerpt) > 600 {
		excerpt = excerpt[:600]
	}
	if excerpt == "" {
		excerpt = a.Title
	}

	sourceInfo := fmt.Sprintf("Topic: %s", topic)
	if a.Title != "" {
		sourceInfo = fmt.Sprintf(`Source: "%s" from %s — %s`, a.Title, a.FeedName, excerpt)
	}

	prompt := fmt.Sprintf(`Write a Twitter thread of 6 tweets for software engineers and PCB designers.
%s

Rules:
- Tweet 1: Hook — bold specific technical claim. End with "🧵"
- Tweets 2–5: One concrete tip, tradeoff, or gotcha each. Numbered (2/, 3/, etc.)
- Tweet 6: Practical takeaway or "save this" closer.
- NEVER claim years of experience or invent personal stories.
- Stay grounded in the source — no invented facts.
- Each tweet <= 250 chars. Use emojis. Add hashtags on last tweet only.
- Output ONLY the 6 tweets, one per line, nothing else.`, sourceInfo)

	raw, err := CallGroqWithSystem(apiKey, creatorSystemPrompt, prompt, 900)
	if err != nil {
		return nil, err
	}

	var tweets []string
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		line = TruncateTweet(line, 280)
		if line != "" {
			tweets = append(tweets, line)
		}
	}
	if len(tweets) < 2 {
		return nil, fmt.Errorf("too few tweets: %d", len(tweets))
	}
	return tweets, nil
}

// GenerateEngagementComment generates a context-aware reply to someone else's tweet.
func GenerateEngagementComment(apiKey, tweetText string) (string, error) {
	prompt := fmt.Sprintf(`Read this tweet and write a natural, human reply:
"%s"

Pick the fitting style:
- Tip/insight → add a related technical point
- Question → answer it directly
- Build/project → react to the specific thing
- Opinion → agree or push back with a reason
- Debugging story → relate or suggest what to check next

Rules:
- Sound like a real engineer, not a bot
- Be specific to what they said
- Only ask a question if it genuinely fits
- Do NOT start with "I"
- Do NOT lecture them
- Max 180 chars. No hashtags. Just the reply text.`, tweetText)

	reply, err := CallGroqWithSystem(apiKey, creatorSystemPrompt, prompt, 80)
	if err != nil {
		return "", err
	}
	return TruncateTweet(reply, 280), nil
}

// IsCreatorTextOnly returns true if the creator format performs better without an image.
func IsCreatorTextOnly(name string) bool {
	return CreatorTextOnly[name]
}

// FetchAndEngage fetches article text and uses Groq to write an engaging tweet.
// Falls back to FormatHeadline if the API key is empty or the request fails.
func FetchAndEngage(a feeds.Article, groqAPIKey string) string {
	if groqAPIKey == "" {
		return feeds.FormatHeadline(a)
	}

	articleText := feeds.FetchText(a.Link)

	var prompt string
	if articleText != "" {
		prompt = fmt.Sprintf(`You are a sharp, opinionated AI and cybersecurity commentator on X (Twitter).

Article title: %s
Source: %s
Excerpt: %s

Write ONE engaging tweet. Rules:
- Strong hook — threat, surprising stat, bold claim, or provocative question
- Frame through AI or security lens
- End with opinion or question to spark replies
- Use 1-3 emojis. Max 260 chars. No hashtags. Just the tweet text.`, a.Title, a.FeedName, articleText)
	} else {
		prompt = fmt.Sprintf(`Article title: %s
Source: %s

Write ONE engaging tweet about this headline. Strong hook, AI/security lens, spark replies.
Max 260 chars. No hashtags. Just the tweet text.`, a.Title, a.FeedName)
	}

	result, err := CallGroq(groqAPIKey, prompt, 150)
	if err != nil {
		return feeds.FormatHeadline(a)
	}
	post := TruncateTweet(result, 280)
	if post == "" {
		return feeds.FormatHeadline(a)
	}
	return post
}
