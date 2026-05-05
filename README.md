# Twitter AI & Security Bot

An automated Twitter/X bot focused on **AI and cybersecurity** content. It monitors curated RSS feeds, generates AI-enhanced tweets, posts memes with AI-generated images, builds reply-chain threads, and self-engages after every post to boost algorithmic reach — completely free using GitHub Actions. No server required.

---

## What It Does

Every 6 hours the bot:
- Polls 40 curated AI + cybersecurity RSS feeds
- Writes engaging AI-powered tweets via Groq (llama-3.3-70b)
- Generates contextual images via Pollinations.ai (free, unlimited)
- Posts with reply-chain threads, link replies, and self-engagement (like + repost + comment)
- Never posts the same article twice

---

## Feed Sets

| File | Feeds | Categories |
|---|---|---|
| `data/ai_security_feeds.json` | 40 | AI, Cybersecurity (default) |
| `data/rss_feeds.json` | 290 | World, Tech, Cybersecurity, Business, Science, Environment, Health, Space, Africa |
| `data/tech_feeds.json` | 143 | Tech, Cybersecurity, Science |

AI sources: OpenAI, Anthropic, DeepMind, Google AI, Hugging Face, MIT Tech Review, VentureBeat, Wired AI, and more.

Security sources: Krebs on Security, BleepingComputer, The Hacker News, Dark Reading, CISA, Schneier on Security, CrowdStrike, Recorded Future, and more.

---

## Post Modes

| Mode | Behaviour |
|---|---|
| `news` | AI-enhanced news tweets + article image + link reply |
| `meme` | AI-generated posts with Pollinations images — 13 formats, 30% threads |
| `mixed` | News articles interleaved with AI meme/thread posts |

### news mode

1. Fetches article content from the source page
2. Groq writes an engaging tweet with a hook, AI/security angle, and opinion
3. Posts with article thumbnail (or Pollinations AI image if none available)
4. Replies with the article link (keeps link off main tweet for better reach)
5. Self-engages: likes, reposts, and comments on the tweet immediately after posting

### meme mode

Groq generates posts in one of 13 formats, all with AI-generated images:

- AI/Security hot takes — "The biggest security threat in 2026 isn't hackers, it's AI"
- Forced choice polls — "Be honest, pick ONE AI tool in 2026: ChatGPT / Claude / Gemini / Copilot"
- Comparison questions — "What's the difference between authentication and authorization?"
- Community hooks — "Drop your GitHub in the comments, let's connect 👇"
- Dev humor, relatable observations, storytelling, educational tips
- Thread starters, open-ended questions, news reactions

30% of meme posts are full 6-tweet reply-chain threads.

All posts get an AI-generated image via Pollinations.ai → memegen.link → Imgflip (fallback chain).

### mixed mode

News articles interleaved with meme/thread posts. One meme injected at the halfway point, with a 33% chance of a second one between news posts.

### Self-engagement

After every post the bot:
1. Likes its own tweet
2. Reposts it
3. Posts a follow-up comment (different style from the original — question if it was a story, fact if it was a question)

This boosts engagement velocity in the critical first 30–60 minutes.

### Threads

Reply-chain threads — tweet 1 posted normally, each subsequent tweet replies to the previous one. True nested thread structure on X.

---

## Image Generation

| Source | Type | Cost |
|---|---|---|
| Pollinations.ai | AI-generated contextual image (Stable Diffusion) | Free, unlimited |
| memegen.link | Meme template with text | Free, unlimited |
| Imgflip | Meme template with text | Free (fallback) |

Groq generates a tailored image prompt from the tweet text before calling Pollinations, so each image is contextually relevant to the post.

---

## Setup Guide

### Step 1 — Fork this repository

Click **Fork** on GitHub.

### Step 2 — Export your Twitter cookies

1. Log into [x.com](https://x.com) in Chrome or Firefox
2. Install [Cookie-Editor](https://cookie-editor.com)
3. Click the icon → **Export** → **Export as JSON**
4. Copy the JSON as a single line

### Step 3 — Add secrets and variables to GitHub

**Settings** → **Secrets and variables** → **Actions**

**Secrets:**

| Secret | Value |
|---|---|
| `TWITTER_USERNAME` | Main account username (without @) |
| `TWITTER_PASSWORD` | Main account password |
| `TWITTER_COOKIES` | Single-line cookie JSON |
| `GROQ_API_KEY` | Free at [console.groq.com](https://console.groq.com) — required |
| `IMGFLIP_USERNAME` | Optional — [imgflip.com](https://imgflip.com) fallback |
| `IMGFLIP_PASSWORD` | Optional |
| `TECH_TWITTER_USERNAME` | Second account (optional) |
| `TECH_TWITTER_PASSWORD` | Second account (optional) |
| `TECH_TWITTER_COOKIES` | Second account (optional) |

**Variables:**

| Variable | Recommended | Description |
|---|---|---|
| `POST_MODE` | `mixed` | `news`, `meme`, or `mixed` |
| `CATEGORY` | _(empty)_ | `ai`, `cybersecurity`, or empty for both |
| `MAX_ARTICLE_AGE_HOURS` | `7` | Cron interval + 1h |
| `TWEET_DELAY_SECONDS` | `90` | Gap between tweets |
| `MAX_TWEETS_PER_RUN` | `5` | Max posts per run |

### Step 4 — Enable GitHub Actions

**Actions** tab → enable workflows → they run every 6 hours automatically.

To test: **Actions** → select workflow → **Run workflow**.

---

## Workflows

| Workflow | Schedule | Feeds | Account |
|---|---|---|---|
| `post.yml` | Every 6 hours | `data/ai_security_feeds.json` | `TWITTER_*` |
| `post_tech.yml` | Every 6 hours | `data/ai_security_feeds.json` | `TECH_TWITTER_*` |

Both: `RUN_ONCE=true`, 30-minute timeout, screenshots uploaded on every run.

---

## Configuration

| Variable | Default | Description |
|---|---|---|
| `TWITTER_USERNAME` | — | Twitter/X username |
| `TWITTER_PASSWORD` | — | Twitter/X password |
| `TWITTER_COOKIES` | — | Session cookies (single-line JSON) |
| `FEEDS_FILE` | `data/ai_security_feeds.json` | Path to feeds file |
| `CATEGORY` | _(all)_ | Filter: `ai` or `cybersecurity` |
| `POST_MODE` | `mixed` | `news`, `meme`, or `mixed` |
| `POLL_INTERVAL_MINUTES` | `5` | Poll interval (continuous mode) |
| `MAX_ARTICLE_AGE_HOURS` | `7` | Ignore articles older than this |
| `TWEET_DELAY_SECONDS` | `90` | Gap between consecutive tweets |
| `MAX_TWEETS_PER_RUN` | `5` | Max tweets per run (0 = unlimited) |
| `RUN_ONCE` | `false` | Poll once then exit (`true` in GitHub Actions) |
| `GROQ_API_KEY` | — | Required for all AI features |
| `IMGFLIP_USERNAME` | — | Optional fallback meme images |
| `IMGFLIP_PASSWORD` | — | Optional |

---

## Running Locally

Requires [Go 1.21+](https://go.dev/dl/) and **Google Chrome**.

```bash
wget https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb
sudo dpkg -i google-chrome-stable_current_amd64.deb && sudo apt-get install -f -y
```

```bash
cp .env.example .env
# fill in TWITTER_COOKIES and GROQ_API_KEY
go run .
```

Single run:
```bash
RUN_ONCE=true go run .
```

---

## How It Works

1. GitHub Actions triggers every 6 hours with `RUN_ONCE=true`
2. Feeds fetched concurrently (15 at a time, 15s timeout each)
3. Articles newer than `MAX_ARTICLE_AGE_HOURS` collected and deduplicated
4. Groq writes an AI-enhanced tweet with AI/security framing
5. Groq generates an image prompt → Pollinations.ai generates the image
6. Tweet posted with image; link posted as reply
7. Bot self-engages: like + repost + follow-up comment
8. In meme mode: 30% chance of a 6-tweet reply-chain thread
9. Headless Chrome handles all browser automation via session cookies
10. Screenshots saved as artifacts on every run

---

## Troubleshooting

**"session invalid or expired"** — Cookies expired. Re-export and update the secret. Cookies last 30–90 days.

**Bot hangs at "Launching browser..."** — Install Google Chrome stable. Snap Chromium doesn't work for headless automation.

**"tweet composer not found"** — Twitter updated their UI. Check debug screenshots in Actions → Summary → Artifacts.

**Same articles every run** — `seen_articles.json` doesn't persist between Actions runs. Keep `MAX_ARTICLE_AGE_HOURS` at cron interval + 1h.

**Thread only posts one tweet** — Check `TWITTER_USERNAME` matches your X handle exactly (used for URL extraction).

---

## License

[MIT](./LICENSE)
