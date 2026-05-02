# Twitter News Bot

An automated Twitter/X bot that monitors RSS feeds, generates AI-enhanced tweets, posts memes, and builds reply-chain threads — completely free using GitHub Actions. No server required.

---

## What It Does

Every 6 hours the bot polls hundreds of RSS feeds, picks unseen articles, and posts them as engaging AI-written tweets. It never posts the same article twice.

**Two feed sets included:**

| File | Feeds | Categories |
|---|---|---|
| `data/rss_feeds.json` | 290 | World, Tech, Cybersecurity, Business, Science, Environment, Health, Space, Africa |
| `data/tech_feeds.json` | 143 | Tech, Cybersecurity, Science |

---

## Post Modes

Set `POST_MODE` to control what the bot posts:

| Mode | Behaviour |
|---|---|
| `news` | AI-enhanced news tweets with image + link reply |
| `meme` | AI-generated humor, hot takes, polls, threads |
| `mixed` | News articles interleaved with AI meme/thread posts |

### news mode

For each article the bot:
1. Fetches the article page and extracts the content
2. Passes the title + excerpt to Groq (llama-3.3-70b) to write an engaging tweet with a hook and opinion
3. Posts the tweet with the article's thumbnail image (no link in the main tweet — better algorithmic reach)
4. Replies to its own tweet with the article link to seed the conversation

### meme mode

Generates AI-powered posts via Groq. 30% of posts are full reply-chain threads (6 tweets). The rest are single posts in one of 10 formats:

- Dev humor — "it works on my machine", merge conflicts, CSS pain
- Hot takes — "Unpopular opinion: tabs are better than spaces"
- Relatable — "me at 9am vs me debugging at 11pm"
- Polls — "🅰️ Vim  🅱️ VS Code"
- Thread starters — "Things nobody tells you about Kubernetes 🧵"
- Questions — open-ended questions that spark replies
- Storytelling — personal dev confessions and stories
- Educational — punchy tips and insights
- News reactions — witty takes on live headlines
- Trending reactions — sarcastic/humorous headline responses

Meme images are generated via [memegen.link](https://memegen.link) (free, no auth) using 20 dev-culture templates. Falls back to Imgflip if memegen fails.

### mixed mode

Interleaves news and meme posts in a single run:
- Posts news articles with AI-written tweets
- Injects one meme/thread at the halfway point
- Randomly injects a second meme between news posts (33% chance)
- Falls back to a standalone meme if no articles are found

### Threads

When the bot posts a thread it uses reply-chaining — tweet 1 is posted normally, then each subsequent tweet replies to the previous one, forming a true nested thread on X.

---

## Setup Guide

### Step 1 — Fork this repository

Click **Fork** on GitHub to copy this repo to your account.

### Step 2 — Export your Twitter cookies

The bot logs into Twitter using browser cookies from your real session.

1. Log into [x.com](https://x.com) in Chrome or Firefox
2. Install the [Cookie-Editor](https://cookie-editor.com) browser extension
3. Click the Cookie-Editor icon → **Export** → **Export as JSON**
4. Copy the JSON — paste it as a single line with no surrounding quotes

### Step 3 — Add secrets and variables to GitHub

Go to your repo → **Settings** → **Secrets and variables** → **Actions**.

**Secrets** (under the Secrets tab):

| Secret | Value |
|---|---|
| `TWITTER_USERNAME` | Main account username (without @) |
| `TWITTER_PASSWORD` | Main account password |
| `TWITTER_COOKIES` | Single-line cookie JSON from Step 2 |
| `GROQ_API_KEY` | Free at [console.groq.com](https://console.groq.com) — required for meme/mixed/news AI |
| `IMGFLIP_USERNAME` | Optional — [imgflip.com](https://imgflip.com) fallback meme images |
| `IMGFLIP_PASSWORD` | Optional — Imgflip password |
| `TECH_TWITTER_USERNAME` | Second account username (optional) |
| `TECH_TWITTER_PASSWORD` | Second account password (optional) |
| `TECH_TWITTER_COOKIES` | Second account cookies (optional) |

**Variables** (under the Variables tab):

| Variable | Recommended value | Description |
|---|---|---|
| `POST_MODE` | `mixed` | `news`, `meme`, or `mixed` |
| `FEEDS_FILE` | `data/rss_feeds.json` | Feed file path |
| `CATEGORY` | _(leave empty)_ | Filter to one category or leave blank for all |
| `MAX_ARTICLE_AGE_HOURS` | `7` | Look back window — set to cron interval + 1h |
| `TWEET_DELAY_SECONDS` | `90` | Gap between tweets |
| `MAX_TWEETS_PER_RUN` | `5` | Max tweets per run |

### Step 4 — Enable GitHub Actions

1. Go to the **Actions** tab in your repository
2. Click **"I understand my workflows, go ahead and enable them"** if prompted
3. Both workflows run automatically every 6 hours

To test immediately: **Actions** → select a workflow → **Run workflow**.

---

## Workflows

| Workflow | Schedule | Feeds | Account secrets |
|---|---|---|---|
| `post.yml` | Every 6 hours | `data/rss_feeds.json` (290 feeds) | `TWITTER_*` |
| `post_tech.yml` | Every 6 hours | `data/tech_feeds.json` (143 feeds) | `TECH_TWITTER_*` |

Both workflows run with `RUN_ONCE=true` (polls once and exits), 20-minute timeout, and upload screenshots on every run as artifacts.

---

## Configuration

All settings are controlled via environment variables. Copy `.env.example` to `.env` for local use.

| Variable | Default | Description |
|---|---|---|
| `TWITTER_USERNAME` | — | Twitter/X username |
| `TWITTER_PASSWORD` | — | Twitter/X password |
| `TWITTER_COOKIES` | — | Session cookies (single-line JSON array) |
| `FEEDS_FILE` | `data/rss_feeds.json` | Path to the feeds file |
| `CATEGORY` | _(all)_ | Filter to one category |
| `POST_MODE` | `news` | `news`, `meme`, or `mixed` |
| `POLL_INTERVAL_MINUTES` | `5` | Poll interval in continuous mode |
| `MAX_ARTICLE_AGE_HOURS` | `7` | Ignore articles older than this |
| `TWEET_DELAY_SECONDS` | `90` | Gap between consecutive tweets |
| `MAX_TWEETS_PER_RUN` | `5` | Max tweets per run (0 = unlimited) |
| `RUN_ONCE` | `false` | Poll once then exit (set `true` in GitHub Actions) |
| `GROQ_API_KEY` | — | Groq API key — required for AI tweet generation |
| `IMGFLIP_USERNAME` | — | Imgflip username (optional fallback for meme images) |
| `IMGFLIP_PASSWORD` | — | Imgflip password |

### Filtering by category

```
CATEGORY=cybersecurity
```

Valid values: `world`, `tech`, `cybersecurity`, `business`, `environment`, `science`, `space`, `health`, `africa`

---

## Customizing Feeds

Each entry in a feeds file:

```json
{
  "name": "Feed Name",
  "category": "tech",
  "url": "https://example.com/feed.rss"
}
```

Any RSS or Atom feed URL works. To create a new feed set, copy `rss_feeds.json` and point `FEEDS_FILE` at it.

---

## Running Locally

Requires [Go 1.21+](https://go.dev/dl/) and **Google Chrome** installed.

On Ubuntu/Debian:
```bash
wget https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb
sudo dpkg -i google-chrome-stable_current_amd64.deb && sudo apt-get install -f -y
```

```bash
cp .env.example .env
# fill in TWITTER_COOKIES, GROQ_API_KEY, and optionally FEEDS_FILE
go run .
```

Single run (same as GitHub Actions):
```bash
RUN_ONCE=true go run .
```

---

## How It Works

1. GitHub Actions triggers every 6 hours with `RUN_ONCE=true`
2. Feeds are fetched concurrently (15 at a time, 15s timeout each)
3. Articles newer than `MAX_ARTICLE_AGE_HOURS` that haven't been seen are collected
4. In `news`/`mixed` mode: article content is fetched and passed to Groq for an AI-written engaging tweet
5. The tweet is posted with the article image; the link is posted as a reply
6. In `meme` mode: Groq generates a post in one of 10 formats; 30% chance of a full reply-chain thread
7. Meme images come from memegen.link (free) with Imgflip as fallback
8. A headless Chrome browser injects session cookies and posts each tweet
9. Screenshots are saved as artifacts on every run

---

## Troubleshooting

**"session invalid or expired"**
Cookies have expired. Re-export from your browser and update the secret. Cookies typically last 30–90 days.

**Bot hangs at "Launching browser..."**
Install Google Chrome stable (see above). Snap Chromium does not work for headless automation.

**"tweet composer not found"**
Twitter updated their UI. Check debug screenshots in the Actions run → **Summary** → **Artifacts**.

**Bot tweets the same articles every run**
`seen_articles.json` doesn't persist between GitHub Actions runs. The bot relies on `MAX_ARTICLE_AGE_HOURS` — keep it set to your cron interval + 1 hour (default `7` for a 6-hour schedule).

**Thread only posts one tweet**
The bot needs a tweet URL to chain replies. If `extractPostedTweetURL` returns empty, replies fall back to the first tweet. Check that your username in `TWITTER_USERNAME` matches your X handle exactly.

---

## License

[MIT](./LICENSE)
