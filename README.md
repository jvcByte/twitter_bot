# Twitter News Bot

An automated Twitter/X bot that monitors RSS feeds across multiple news categories and tweets breaking news as it happens — completely free using GitHub Actions. No server required. Supports multiple Twitter accounts with different feed sets.

---

## What It Does

Every 6 hours, the bot polls all feeds concurrently, finds articles published since the last run that haven't been tweeted yet, and posts them one by one. It never tweets the same article twice.

**Two feed sets included:**

| File | Feeds | Categories |
|---|---|---|
| `data/rss_feeds.json` | 290 | World, Tech, Cybersecurity, Business, Science, Environment, Health, Space, Africa |
| `data/tech_feeds.json` | 143 | Tech, Cybersecurity, Science |

**Coverage highlights:**

| Category | Sources |
|---|---|
| World | Reuters, BBC, AP, Al Jazeera, Guardian, NYT, Washington Post, CNN, and 50+ more |
| Tech | TechCrunch, The Verge, Wired, Ars Technica, VentureBeat, OpenAI, GitHub, Nvidia, and 75+ more |
| Cybersecurity | Krebs on Security, BleepingComputer, CISA, CrowdStrike, Google Security, and 25+ more |
| Business | Bloomberg, Financial Times, WSJ, Forbes, CNBC, The Economist, and 30+ more |
| Science | Nature, Quanta, MIT News, Scientific American, arXiv, and 20+ more |
| Environment | Carbon Brief, CleanTechnica, Inside Climate News, Electrek, and 20+ more |
| Health | STAT News, The Lancet, BMJ, Fierce Healthcare, and 10+ more |
| Space | Space.com, NASA, ESA, SpaceflightNow, The Planetary Society, and more |
| Africa | TechCabal, AllAfrica, The Africa Report, Nairametrics, and more |

---

## Prerequisites

- [GitHub](https://github.com) — runs the bot for free
- One or more [Twitter/X](https://x.com) accounts to post to

---

## Setup Guide

### Step 1 — Fork this repository

Click **Fork** on GitHub to copy this repo to your account.

### Step 2 — Export your Twitter cookies

The bot logs into Twitter using browser cookies from your real session to avoid bot detection.

1. Log into [x.com](https://x.com) in Chrome or Firefox
2. Install the [Cookie-Editor](https://cookie-editor.com) browser extension
3. Click the Cookie-Editor icon → **Export** → **Export as JSON**
4. Copy the JSON — it must be pasted as a single line with no surrounding quotes

### Step 3 — Add secrets to GitHub

Go to your repo → **Settings** → **Secrets and variables** → **Actions** → **New repository secret**.

**Main account** (used by `post.yml` — all 290 feeds):

| Secret | Value |
|---|---|
| `TWITTER_USERNAME` | Username (without @) |
| `TWITTER_PASSWORD` | Password |
| `TWITTER_COOKIES` | Single-line cookie JSON from Step 2 |

**Tech account** (used by `post_tech.yml` — 143 tech/cyber/science feeds):

| Secret | Value |
|---|---|
| `TECH_TWITTER_USERNAME` | Username (without @) |
| `TECH_TWITTER_PASSWORD` | Password |
| `TECH_TWITTER_COOKIES` | Single-line cookie JSON from Step 2 |

If you only want one account, only add the main account secrets and disable `post_tech.yml`.

**Optional — for meme/mixed mode** (shared across both workflows):

| Secret | Value |
|---|---|
| `GROQ_API_KEY` | Free at [console.groq.com](https://console.groq.com) |
| `IMGFLIP_USERNAME` | Free at [imgflip.com](https://imgflip.com) |
| `IMGFLIP_PASSWORD` | Your Imgflip password |

### Step 4 — Enable GitHub Actions

1. Go to the **Actions** tab in your repository
2. If prompted, click **"I understand my workflows, go ahead and enable them"**
3. Both workflows will now run automatically every 6 hours

To test immediately: **Actions** → select a workflow → **Run workflow**.

---

## Workflows

| Workflow | Schedule | Feeds | Mode | Account secrets |
|---|---|---|---|---|
| `post.yml` | Every 6 hours | `data/rss_feeds.json` (290 feeds, all categories) | `news` | `TWITTER_*` |
| `post_tech.yml` | Every 6 hours | `data/tech_feeds.json` (143 feeds, tech/cyber/science) | `mixed` | `TECH_TWITTER_*` |

---

## Configuration

All settings are controlled via environment variables. Copy `.env.example` to `.env` for local use.

| Variable | Default | Description |
|---|---|---|
| `TWITTER_USERNAME` | — | Twitter/X username |
| `TWITTER_PASSWORD` | — | Twitter/X password |
| `TWITTER_COOKIES` | — | Session cookies (single-line JSON array) |
| `FEEDS_FILE` | `data/rss_feeds.json` | Path to the feeds file to use |
| `CATEGORY` | _(all)_ | Filter to one category within the feeds file |
| `POST_MODE` | `news` | Content mode: `news`, `meme`, or `mixed` |
| `POLL_INTERVAL_MINUTES` | `5` | Poll interval in continuous mode |
| `MAX_ARTICLE_AGE_HOURS` | `7` | Ignore articles older than this |
| `TWEET_DELAY_SECONDS` | `90` | Gap between consecutive tweets |
| `MAX_TWEETS_PER_RUN` | `5` | Max tweets per run (0 = unlimited) |
| `RUN_ONCE` | `false` | Exit after one poll — set `true` in CI |
| `GROQ_API_KEY` | — | Groq API key for AI post generation (free at [console.groq.com](https://console.groq.com)) |
| `IMGFLIP_USERNAME` | — | Imgflip username for meme images (free at [imgflip.com](https://imgflip.com)) |
| `IMGFLIP_PASSWORD` | — | Imgflip password |

### Filtering by category

Set `CATEGORY` to focus on a single topic within whichever `FEEDS_FILE` you're using:

```
CATEGORY=cybersecurity
```

Valid values: `world`, `tech`, `cybersecurity`, `business`, `environment`, `science`, `space`, `health`, `africa`

---

## Post Modes

Control what the bot posts with `POST_MODE`:

| Mode | Behaviour |
|---|---|
| `news` | News articles only with article thumbnail images when available (default) |
| `meme` | AI-generated humor posts with Imgflip meme images |
| `mixed` | News articles + one AI meme post injected per run |

### Meme post formats (AI-generated via Groq)

- Dev humor — "it works on my machine", merge conflicts, CSS pain
- Hot takes — "Unpopular opinion: tabs are better than spaces"
- Relatable observations — "me at 9am vs me debugging at 11pm"
- Polls — "🅰️ Vim  🅱️ VS Code"
- Thread starters — "Things nobody tells you about Kubernetes 🧵"
- Trending reactions — reacts to a live headline with dev humor

### Meme images (via Imgflip)

When `IMGFLIP_USERNAME` and `IMGFLIP_PASSWORD` are set, meme posts get a generated image using classic templates: Drake, Distracted Boyfriend, Expanding Brain, UNO Draw 25, Two Buttons, and more. Falls back to text-only if credentials are missing.

---

## Customizing Feeds

Each entry in a feeds file has three fields:

```json
{
  "name": "Feed Name",
  "category": "tech",
  "url": "https://example.com/feed.rss"
}
```

Any RSS or Atom feed URL works. To create a new feed set, copy `rss_feeds.json`, filter or add entries, and point `FEEDS_FILE` at it.

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
# fill in credentials, TWITTER_COOKIES, and optionally FEEDS_FILE
go run .
```

Single run (same behaviour as GitHub Actions):
```bash
RUN_ONCE=true go run .
```

---

## Troubleshooting

**"session invalid or expired"**
Cookies have expired. Re-export from your browser and update the secret. Cookies typically last 30–90 days.

**Bot hangs at "Launching browser..."**
Install Google Chrome stable (see above). On Ubuntu 24.04, snap Chromium does not work for headless automation.

**"tweet composer not found"**
Twitter updated their UI. Check debug screenshots in the failed Actions run → **Summary** → **Artifacts**.

**Bot tweets the same articles every run**
`seen_articles.json` doesn't persist between GitHub Actions runs. The bot relies on `MAX_ARTICLE_AGE_HOURS` to avoid re-tweeting — keep it set to your cron interval + 1 hour (default `7` for a 6-hour schedule).

---

## How It Works

1. GitHub Actions triggers on cron every 6 hours (`RUN_ONCE=true`)
2. The bot loads feeds from the configured `FEEDS_FILE`
3. All feeds are fetched concurrently (15 at a time) with a 15s timeout each
4. Articles newer than `MAX_ARTICLE_AGE_HOURS` that haven't been seen are collected
5. Results are sorted newest-first, capped at `MAX_TWEETS_PER_RUN`, and tweeted with a `TWEET_DELAY_SECONDS` gap
6. A headless Chrome browser injects session cookies, navigates to x.com, and posts each tweet
7. Screenshots are saved automatically on failure

---

## License

[MIT](./LICENSE)
