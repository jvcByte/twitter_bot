# Twitter AI Bot

An automated Twitter/X bot for Software Engineers and PCB Designers. Monitors curated RSS feeds, generates AI-enhanced tweets, posts owned engineering content grounded in real articles, and engages with the community — all free via GitHub Actions. No server required.

---

## What It Does

The bot runs on a schedule and rotates through five modes:

- **News** — Polls 40+ RSS feeds, writes AI-enhanced tweets with images and link replies
- **Creator** — Generates owned technical content (code tips, PCB insights, dev opinions) grounded in real articles from dev/embedded feeds
- **Meme** — AI-generated posts in 13 formats with Pollinations images — hot takes, polls, threads
- **Engage** — Searches X for relevant topics, likes posts, comments with context-aware replies, occasionally reposts
- **Mixed** — Rotates through all modes: `news → creator → engage → news → meme → creator → engage → news`

After every post the bot self-engages (like + repost + follow-up comment) to boost reach in the first 30–60 minutes.

---

## Project Structure

```
cmd/bot/              — entry point (go run ./cmd/bot/)
internal/
  config/             — environment variable loading
  feeds/              — RSS polling, deduplication, article fetching
  generation/         — Groq LLM calls, meme formats, creator content, engagement comments
  images/             — image generation (Pollinations → memegen → Imgflip)
  twitter/            — headless browser automation (tweet, reply, thread, engage)
  bot/                — orchestration: modes, rotation, pipelines
data/
  ai_security_feeds.json   — 40 AI + cybersecurity feeds (default)
  dev_feeds.json            — 45 dev + embedded engineering feeds (for creator mode)
  rss_feeds.json            — 290 feeds across 9 categories
  tech_feeds.json           — 143 tech + cybersecurity feeds
  seen_articles.json        — deduplication store (committed, persists across runs)
  seen_creator.json         — creator article dedup store
  rotation_state.json       — current rotation slot (persists across runs)
.github/workflows/
  post.yml            — main account, every 6h
  post_tech.yml       — second account, every 6h
  engage.yml          — engagement only, every 30 minutes
```

---

## Post Modes

| Mode | Description |
|---|---|
| `news` | AI-enhanced news tweets + image + link reply + self-engage |
| `meme` | 13 AI post formats, 30% threads, Pollinations images |
| `creator` | Owned content from real dev/embedded articles — no fabricated facts |
| `engage` | Like + comment + occasional repost on relevant posts |
| `mixed` | 8-slot rotation: 3 news, 2 creator, 1 meme, 2 engage per day |

### Creator Mode

Pulls real articles from 45 dev/embedded feeds (DEV.to, Hashnode, HN, Interrupt/Memfault, EmbeddedRelated, Hackaday, EEVblog, IEEE Spectrum, etc.) and generates tweets grounded in the actual content.

Seven formats: code tips, PCB/embedded tips, dev opinions, TIL moments, tool takes, career insights, honest confessions.

Rules enforced by prompt:
- Never claims specific years of experience
- Never invents personal projects or company history
- Must name specific tools, languages, or components
- BAD/GOOD examples in every prompt to enforce specificity

### Engagement Mode

Searches X for topics like `PCB design`, `#EmbeddedSystems`, `#Golang`, `firmware development`, `AI cybersecurity`, etc. For each post found:
1. Likes the post
2. Generates a context-aware comment (tip if it's a tip, answer if it's a question, reaction if it's a hot take)
3. ~20% chance of reposting

### Self-engagement

After every post the bot likes, reposts, and replies with a follow-up comment in a different style from the original — this seeds the reply chain and boosts algorithmic velocity.

---

## Image Generation

| Source | Type | Cost |
|---|---|---|
| Pollinations.ai | AI-generated contextual image (Stable Diffusion) | Free, unlimited |
| memegen.link | Meme template | Free, unlimited |
| Imgflip | Meme template (fallback) | Free |

Groq generates a tailored image prompt from the tweet text before calling Pollinations.

---

## Setup

### 1. Fork this repo

Click **Fork** on GitHub.

### 2. Export Twitter cookies

1. Log into [x.com](https://x.com) in Chrome or Firefox
2. Install [Cookie-Editor](https://cookie-editor.com)
3. Click the icon → **Export** → **Export as JSON**
4. Copy the JSON as a single line

### 3. Add GitHub secrets

**Settings → Secrets and variables → Actions → Secrets**

| Secret | Required | Description |
|---|---|---|
| `TWITTER_USERNAME` | ✅ | Username without @ |
| `TWITTER_PASSWORD` | ✅ | Account password |
| `TWITTER_COOKIES` | ✅ | Single-line cookie JSON |
| `GROQ_API_KEY` | ✅ | Free at [console.groq.com](https://console.groq.com) |
| `IMGFLIP_USERNAME` | Optional | Fallback meme images |
| `IMGFLIP_PASSWORD` | Optional | |
| `TECH_TWITTER_USERNAME` | Optional | Second account |
| `TECH_TWITTER_PASSWORD` | Optional | |
| `TECH_TWITTER_COOKIES` | Optional | |

### 4. Set repository variables

**Settings → Secrets and variables → Actions → Variables**

| Variable | Recommended | Description |
|---|---|---|
| `POST_MODE` | `mixed` | `news`, `meme`, `creator`, `engage`, or `mixed` |
| `CATEGORY` | _(empty)_ | `ai`, `cybersecurity`, or empty for both |
| `MAX_ARTICLE_AGE_HOURS` | `7` | Match cron interval + 1h buffer |
| `TWEET_DELAY_SECONDS` | `90` | Gap between tweets |
| `MAX_TWEETS_PER_RUN` | `5` | Max posts per run (0 = unlimited) |

### 5. Enable Actions write permissions

**Settings → Actions → General → Workflow permissions → Read and write**

Required for the bot to commit state files (`seen_articles.json`, `rotation_state.json`) after each run.

### 6. Enable workflows

**Actions** tab → enable workflows → they run automatically on schedule.

To test: **Actions** → select workflow → **Run workflow**.

---

## Workflows

| Workflow | Schedule | Mode | Account |
|---|---|---|---|
| `post.yml` | Every 6h | `POST_MODE` var | `TWITTER_*` |
| `post_tech.yml` | Every 6h | `POST_MODE` var | `TECH_TWITTER_*` |
| `engage.yml` | Every 30 min | `engage` (hardcoded) | `TWITTER_*` |

All workflows commit state files back to the repo after each run so the rotation slot and seen-article list persist across runs.

---

## Running Locally

Requires [Go 1.21+](https://go.dev/dl/) and Google Chrome:

```bash
wget https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb
sudo dpkg -i google-chrome-stable_current_amd64.deb && sudo apt-get install -f -y
```

```bash
cp .env.example .env
# fill in credentials
go run ./cmd/bot/
```

Single run:
```bash
RUN_ONCE=true go run ./cmd/bot/
```

Run only engagement:
```bash
POST_MODE=engage RUN_ONCE=true go run ./cmd/bot/
```

---

## Configuration Reference

| Variable | Default | Description |
|---|---|---|
| `TWITTER_USERNAME` | — | Twitter/X username |
| `TWITTER_PASSWORD` | — | Twitter/X password |
| `TWITTER_COOKIES` | — | Session cookies JSON |
| `FEEDS_FILE` | `data/ai_security_feeds.json` | RSS feeds file |
| `CATEGORY` | _(all)_ | `ai` or `cybersecurity` |
| `POST_MODE` | `news` | `news`, `meme`, `creator`, `engage`, `mixed` |
| `POLL_INTERVAL_MINUTES` | `5` | Loop interval (continuous mode only) |
| `MAX_ARTICLE_AGE_HOURS` | `7` | Ignore articles older than this |
| `TWEET_DELAY_SECONDS` | `90` | Gap between tweets |
| `MAX_TWEETS_PER_RUN` | `5` | 0 = unlimited |
| `GROQ_API_KEY` | — | Required for all AI features |
| `IMGFLIP_USERNAME` | — | Optional |
| `IMGFLIP_PASSWORD` | — | Optional |
| `RUN_ONCE` | `false` | Exit after one poll (used in CI) |

---

## Troubleshooting

**"session invalid or expired"** — Cookies expired. Re-export from x.com and update the secret. Cookies last 30–90 days.

**Bot hangs at "Launching browser..."** — Google Chrome stable must be installed. Snap Chromium doesn't work for headless automation.

**"tweet composer not found"** — X updated their UI selectors. Check debug screenshots in Actions → run summary → Artifacts.

**Same articles every run** — The state files (`seen_articles.json`, `rotation_state.json`) must be committed back after each run. Check that workflow permissions are set to read/write.

**Thread only posts the first tweet** — `TWITTER_USERNAME` must match your X handle exactly — it's used to identify the posted tweet URL in the timeline.

---

## License

[MIT](./LICENSE)
