package generation

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// PostFormat defines a tweet personality format for meme/AI posts.
type PostFormat struct {
	Name   string
	Prompt string
}

// TextOnlyFormats are engagement-bait formats that perform better without images.
var TextOnlyFormats = map[string]bool{
	"forced_choice_poll":  true,
	"comparison_question": true,
	"community_hook":      true,
}

var memeFormats = []PostFormat{
	{Name: "dev_humor", Prompt: `Write a single funny tweet about software development or programming.
Style: relatable dev humor like "it works on my machine", merge conflicts, CSS pain,
Monday deploys, or debugging at 3am. Use emojis. Max 230 chars. Just the tweet text.`},
	{Name: "hot_take", Prompt: `Write a single spicy tech hot take tweet starting with "Unpopular opinion:" or "Hot take:".
Make it about AI, cybersecurity, software development, or tech culture.
Should be slightly controversial but not offensive. Max 230 chars. Just the tweet text.`},
	{Name: "relatable", Prompt: `Write a single relatable tweet for developers/tech people using the "me at X vs me at Y" format
or "nobody: / developers:" format. About coding, debugging, meetings, deadlines, or tech life.
Use emojis. Max 230 chars. Just the tweet text.`},
	{Name: "forced_choice_poll", Prompt: `Write a single "be honest, pick ONE" style tweet for developers. Format:
[Topic]... be honest

Pick ONE in 2026:
[Option A]
[Option B]
[Option C]
[Option D]

Drop your pick 👇

Topics: AI tools (ChatGPT vs Claude vs Gemini vs Copilot), frontend frameworks, backend languages,
cloud providers, editors, databases, security tools. Max 230 chars. Just the tweet text.`},
	{Name: "comparison_question", Prompt: `Write a single "what's the difference between X and Y?" tweet that sparks discussion.
Pick two things developers often confuse: AI concepts, security terms, frameworks, patterns, tools.
Keep it short and punchy. Use emojis. Max 230 chars. Just the tweet text.`},
	{Name: "community_hook", Prompt: `Write a single community-building tweet for tech/developer Twitter. Format:
[Engaging opener about AI, security, or coding]

Drop your [X] in the comments 👇
[Simple call to action like "follow 3 people who reply" or "let's connect"]

Make it feel warm and community-driven. Use emojis. Max 230 chars. Just the tweet text.`},
	{Name: "ai_security_take", Prompt: `Write a single punchy tweet about AI or cybersecurity that will spark replies.
Could be a warning, a surprising fact, a prediction, or a strong opinion.
Use emojis. Max 230 chars. Just the tweet text.`},
	{Name: "thread_starter", Prompt: `Write a single tweet that starts a thread about AI or cybersecurity with "🧵" at the end.
Format: "X things [about AI/security topic] that will blow your mind 🧵"
Use emojis. Max 230 chars. Just the tweet text.`},
	{Name: "reaction", Prompt: `Given this tech headline: "%s"
Write a single funny/witty tweet reacting to it from a developer's perspective.
Use emojis. Max 230 chars. Just the tweet text.`},
	{Name: "question", Prompt: `Write a single open-ended question tweet for developers/tech people focused on AI or security.
Should spark debate. Use emojis. Max 230 chars. Just the tweet text.`},
	{Name: "storytelling", Prompt: `Write a single tweet that opens a relatable developer story about AI or security.
Format: start with "Story time:" or "True story:" or a punchy hook.
Use emojis. Max 230 chars. Just the tweet text.`},
	{Name: "educational", Prompt: `Write a single punchy educational tweet about AI or cybersecurity.
Share one genuinely useful tip or insight. Lead with the insight.
Use emojis. Max 230 chars. Just the tweet text.`},
	{Name: "news_reaction", Prompt: `Given this tech headline: "%s"
Write a single tweet reacting to it with a strong opinion — agree, disagree, or add context.
Use emojis. Max 230 chars. Just the tweet text.`},
}

// GenerateMemePost generates an AI-powered tweet in a random format.
// headline is optional — used for reaction/news_reaction formats.
func GenerateMemePost(apiKey, headline string) (string, string, error) {
	rand.Seed(time.Now().UnixNano())

	available := memeFormats
	if headline == "" {
		var filtered []PostFormat
		for _, f := range memeFormats {
			if f.Name != "reaction" && f.Name != "news_reaction" {
				filtered = append(filtered, f)
			}
		}
		available = filtered
	}
	format := available[rand.Intn(len(available))]

	prompt := format.Prompt
	if (format.Name == "reaction" || format.Name == "news_reaction") && headline != "" {
		prompt = fmt.Sprintf(format.Prompt, headline)
	}

	post, err := CallGroq(apiKey, prompt, 150)
	if err != nil {
		return "", "", err
	}
	return TruncateTweet(post, 280), format.Name, nil
}

// GenerateSelfComment generates a short follow-up comment for self-engagement.
func GenerateSelfComment(apiKey, postText string) string {
	if apiKey == "" {
		return ""
	}
	prompt := fmt.Sprintf(`You just posted this tweet:
"%s"

Write a SHORT follow-up comment (1 sentence) to reply to your own tweet.
Rules:
- Must be DIFFERENT in style from the original
- Goal: spark replies from other people
- Use 1 emoji max
- Max 180 chars. Just the comment text.`, postText)

	result, err := CallGroq(apiKey, prompt, 60)
	if err != nil {
		return ""
	}
	return TruncateTweet(result, 280)
}

// GenerateThread generates a 6-tweet thread on an AI/security topic.
func GenerateThread(apiKey, topic string) ([]string, error) {
	topicLine := "Pick an interesting AI or cybersecurity topic that developers care about."
	if topic != "" {
		topicLine = fmt.Sprintf("Topic: %s", topic)
	}

	prompt := fmt.Sprintf(`Write a Twitter thread of 6 tweets for developers and security professionals.
%s

Rules:
- Tweet 1: Hook — bold claim or shocking fact. End with "🧵"
- Tweets 2–5: One concrete insight or tip each. Numbered (2/, 3/, etc.)
- Tweet 6: Strong closer — call to action or "save this" moment.
- Each tweet <= 260 chars. Use emojis. Add 1-2 hashtags.
- Output ONLY the 6 tweets, one per line, nothing else.`, topicLine)

	raw, err := CallGroq(apiKey, prompt, 900)
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
		return nil, fmt.Errorf("too few thread tweets: %d", len(tweets))
	}
	return tweets, nil
}

// IsTextOnly returns true if the format performs better without an image.
func IsTextOnly(formatName string) bool {
	return TextOnlyFormats[formatName]
}
