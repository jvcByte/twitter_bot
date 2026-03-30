# Twitter Tech Bot 🤖

An automated Twitter/X bot that posts tech content 5 times a day — completely free using GitHub Actions. No server required.

---

## What It Does

Every few hours, the bot wakes up, picks one of three content types at random, and posts it to your Twitter/X account:

- 📰 Latest headlines from TechCrunch, Hacker News, The Verge, Ars Technica, and Dev.to
- 💡 Tech quotes, tips, facts, and dev humor from a built-in template library
- 🤖 AI-generated tech insights powered by Hugging Face

---

## Prerequisites

You need accounts on three platforms before starting:

- [GitHub](https://github.com) — runs the bot for free
- [Twitter/X](https://x.com) — the account the bot will post to
- [Hugging Face](https://huggingface.co) — provides the AI text generation

---

## Setup Guide

### Step 1 — Fork or clone this repository

Click the **Fork** button on GitHub to copy this repo to your account. That's the version you'll configure and run.

### Step 2 — Get a Hugging Face API token

1. Sign up or log in at [huggingface.co](https://huggingface.co)
2. Click your profile picture → **Settings** → **Access Tokens**
3. Click **New token**
4. Give it any name, select **Read** role, and make sure **"Make calls to the serverless Inference API"** is checked
5. Copy the token — it starts with `hf_`

### Step 3 — Export your Twitter cookies

The bot logs into Twitter using browser cookies from your real session. This avoids bot detection.

1. Log into [x.com](https://x.com) in Chrome or Firefox
2. Install the [Cookie-Editor](https://cookie-editor.com) browser extension
3. Click the Cookie-Editor icon in your toolbar
4. Click **Export** → **Export as JSON**
5. This copies the cookies to your clipboard — keep it ready for Step 4

### Step 4 — Add secrets to GitHub

In your forked repository on GitHub:

1. Go to **Settings** → **Secrets and variables** → **Actions**
2. Click **New repository secret** and add each of the following:

| Secret name | Value |
|---|---|
| `TWITTER_USERNAME` | Your Twitter/X username (without @) |
| `TWITTER_PASSWORD` | Your Twitter/X password |
| `TWITTER_COOKIES` | The full JSON you copied in Step 3 |
| `HUGGINGFACE_API_KEY` | Your Hugging Face token from Step 2 |

### Step 5 — Enable GitHub Actions

1. Go to the **Actions** tab in your repository
2. If prompted, click **"I understand my workflows, go ahead and enable them"**
3. The bot will now run automatically 5 times per day

To test it immediately, go to **Actions** → **Post Tweet** → **Run workflow** → **Run workflow**.

---

## Posting Schedule

The bot runs at these times (UTC) every day:

| Run | UTC time |
|---|---|
| Morning | 8:00 AM |
| Midday | 12:00 PM |
| Afternoon | 4:00 PM |
| Evening | 8:00 PM |
| Night | 11:00 PM |

To change the schedule, edit the cron expressions in `.github/workflows/post.yml`.

---

## Customization

### Add your own templates

Open `data/templates.json` and add entries following this format:

```json
{
  "type": "tip",
  "content": "Your tweet text here\n\n#YourHashtag"
}
```

Types can be anything: `quote`, `tip`, `fact`, `meme`.

### Add or remove RSS feeds

Open `data/rss_feeds.json` and add entries:

```json
{
  "name": "Feed Name",
  "url": "https://example.com/feed.rss"
}
```

Any RSS or Atom feed URL works.

---

## Running Locally

If you want to run the bot on your own machine:

1. Install [Go 1.21+](https://go.dev/dl/)
2. Clone the repository
3. Copy the example env file and fill in your credentials:

```bash
cp .env.example .env
```

Edit `.env`:

```
TWITTER_USERNAME=your_username
TWITTER_PASSWORD=your_password
HUGGINGFACE_API_KEY=hf_your_token
```

4. Export your Twitter cookies to `cookies.json` in the project root (same process as Step 3 above)

5. Run the bot:

```bash
go run .
```

---

## Troubleshooting

**The workflow fails with "session invalid or expired"**
Your Twitter cookies have expired. Repeat Step 3 and update the `TWITTER_COOKIES` secret. Cookies typically last 30–90 days.

**The workflow fails with "tweet composer not found"**
Twitter may have updated their page structure. Open an issue with the error and any screenshots from the artifacts.

**The AI post fails**
Your Hugging Face token may have expired or lost its Inference API permission. Generate a new token and update the `HUGGINGFACE_API_KEY` secret.

**I want to see what the browser sees**
When the workflow fails, GitHub automatically uploads debug screenshots as artifacts. Go to the failed Actions run → **Summary** → **Artifacts** → download `debug-screenshots`.

---

## How It Works (technical)

1. GitHub Actions triggers the workflow on a cron schedule
2. The bot picks a random content type (template / RSS / AI)
3. For AI posts, it calls the Hugging Face Inference API
4. For RSS posts, it fetches the latest item from a random feed
5. It launches a headless Chromium browser, injects your session cookies, navigates to x.com, and posts the tweet
6. Screenshots are saved for debugging if anything goes wrong

---

## License

MIT
