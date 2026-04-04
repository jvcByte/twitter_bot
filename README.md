# Twitter News Bot

An automated Twitter/X bot that monitors **290 RSS feeds** across 9 news categories and tweets breaking news as it happens — completely free using GitHub Actions. No server required.

---

## What It Does

Every 2 hours, the bot polls all 290 feeds concurrently, finds articles published in the last 3 hours that haven't been tweeted yet, and posts them one by one. It never tweets the same article twice.

**Coverage across 9 categories:**

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
- [Twitter/X](https://x.com) — the account the bot posts to

---

## Setup Guide

### Step 1 — Fork this repository

Click **Fork** on GitHub to copy this repo to your account.

### Step 2 — Export your Twitter cookies

The bot logs into Twitter using browser cookies from your real session to avoid bot detection.

1. Log into [x.com](https://x.com) in Chrome or Firefox
2. Install the [Cookie-Editor](https://cookie-editor.com) browser extension
3. Click the Cookie-Editor icon → **Export** → **Export as JSON**
4. This copies the cookie JSON to your clipboard

### Step 3 — Add secrets to GitHub

Go to your forked repo → **Settings** → **Secrets and variables** → **Actions** → **New repository secret**:

| Secret | Value |
|---|---|
| `TWITTER_USERNAME` | Your Twitter/X username (without @) |
| `TWITTER_PASSWORD` | Your Twitter/X password |
| `TWITTER_COOKIES` | The full JSON from Step 2 |

### Step 4 — Enable GitHub Actions

1. Go to the **Actions** tab in your repository
2. If prompted, click **"I understand my workflows, go ahead and enable them"**
3. The bot will now run automatically every 2 hours

To test immediately: **Actions** → **Post Tweet** → **Run workflow**.

---

## Posting Schedule

Runs every 2 hours via cron (`0 */2 * * *`). Each run looks back 3 hours to ensure no articles are missed between runs.

To change the schedule, edit the cron expression in `.github/workflows/post.yml`.

---

## Configuration

All settings are controlled via environment variables. Copy `.env.example` to `.env` for local use, or set them as GitHub Actions secrets/variables.

| Variable | Default | Description |
|---|---|---|
| `TWITTER_USERNAME` | — | Your Twitter/X username |
| `TWITTER_PASSWORD` | — | Your Twitter/X password |
| `TWITTER_COOKIES` | — | Session cookies JSON (single-line JSON array) |
| `CATEGORY` | _(all)_ | Filter to one category (see below) |
| `POLL_INTERVAL_MINUTES` | `5` | Feed poll interval (continuous mode only) |
| `MAX_ARTICLE_AGE_HOURS` | `2` | Ignore articles older than this |
| `TWEET_DELAY_SECONDS` | `90` | Gap between consecutive tweets |
| `MAX_TWEETS_PER_RUN` | `5` | Max tweets per run (0 = unlimited) |
| `RUN_ONCE` | `false` | Exit after one poll — set to `true` in CI |

### Filtering by category

Set `CATEGORY` to focus on a single topic:

```
CATEGORY=cybersecurity
```

Valid values: `world`, `tech`, `cybersecurity`, `business`, `environment`, `science`, `space`, `health`, `africa`

---

## Customizing Feeds

Open `data/rss_feeds.json`. Each entry has three fields:

```json
{
  "name": "Feed Name",
  "category": "tech",
  "url": "https://example.com/feed.rss"
}
```

Any RSS or Atom feed URL works. Categories must match one of the 9 values listed above.

---

## Running Locally

Requires [Go 1.21+](https://go.dev/dl/) and **Google Chrome** or Chromium installed.

On Ubuntu/Debian:
```bash
wget https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb
sudo dpkg -i google-chrome-stable_current_amd64.deb && sudo apt-get install -f -y
```

### Continuous mode (polls forever)

```bash
cp .env.example .env
# fill in credentials and TWITTER_COOKIES
go run .
```

### Single run then exit (same as GitHub Actions)

```bash
RUN_ONCE=true go run .
```

---

## Troubleshooting

**"session invalid or expired"**
Your Twitter cookies have expired. Repeat Step 2 and update the `TWITTER_COOKIES` secret. Cookies typically last 30–90 days.

**Bot hangs at "Launching browser..."**
No working Chrome/Chromium binary was found or the installed version doesn't support headless mode. Install Google Chrome stable (see Running Locally above). On Ubuntu 24.04, snap Chromium does not work for headless automation.

**"no Chromium/Chrome binary found"**
Install Google Chrome stable as shown above.

**"tweet composer not found"**
Twitter may have updated their page structure. Check the debug screenshots in the failed Actions run → **Summary** → **Artifacts** → `debug-screenshots`.

**Bot tweets the same articles every run**
The `data/seen_articles.json` deduplication file doesn't persist between GitHub Actions runs by default. The bot relies on `MAX_ARTICLE_AGE_HOURS` to avoid re-tweeting — set it to match your cron interval (e.g. `2` for a 2-hour schedule) so articles age out naturally between runs.

---

## How It Works

1. GitHub Actions triggers on cron every 2 hours (`RUN_ONCE=true`)
2. The bot loads all 290 feeds from `data/rss_feeds.json`
3. All feeds are fetched concurrently (15 at a time) with a 15-second timeout each
4. Articles newer than `MAX_ARTICLE_AGE_HOURS` that haven't been seen are collected
5. Results are sorted newest-first and tweeted one by one with a `TWEET_DELAY_SECONDS` gap
6. A headless Chromium browser injects your session cookies, navigates to x.com, and posts each tweet
7. Screenshots are saved automatically if anything goes wrong

---

## License

[MIT](./LICENSE)
