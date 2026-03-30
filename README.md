# Twitter Tech Bot 🤖

An automated Twitter bot that posts tech-related content 5 times daily using GitHub Actions.

## Features

- 📰 Posts from tech RSS feeds (TechCrunch, Hacker News, The Verge, etc.)
- 💡 Shares predefined tech quotes, tips, and memes
- 🤖 Generates AI-powered tech insights using Hugging Face
- ⏰ Runs automatically 5 times per day via GitHub Actions
- 🆓 Completely free hosting on GitHub

## Setup

### 1. Get Twitter API Credentials

1. Go to [developer.twitter.com](https://developer.twitter.com)
2. Create a new app
3. Generate API keys and access tokens
4. Save these credentials (you'll need them for GitHub Secrets)

### 2. Get Hugging Face API Key

1. Sign up at [huggingface.co](https://huggingface.co)
2. Go to Settings → Access Tokens
3. Create a new token

### 3. Configure GitHub Secrets

In your GitHub repository, go to Settings → Secrets and variables → Actions, then add:

- `TWITTER_API_KEY`
- `TWITTER_API_SECRET`
- `TWITTER_ACCESS_TOKEN`
- `TWITTER_ACCESS_SECRET`
- `HUGGINGFACE_API_KEY`

### 4. Enable GitHub Actions

1. Go to the Actions tab in your repository
2. Enable workflows if prompted
3. The bot will run automatically at scheduled times

## Local Testing

```bash
# Copy environment template
cp .env.example .env

# Add your credentials to .env
nano .env

# Install dependencies
go mod download

# Run the bot
go run main.go
```

## Customization

### Change Posting Schedule

Edit `.github/workflows/post.yml` and modify the cron expressions.

### Add More Templates

Edit `data/templates.json` to add your own quotes, tips, or memes.

### Add More RSS Feeds

Edit `data/rss_feeds.json` to include additional tech news sources.

## How It Works

1. GitHub Actions triggers the workflow 5 times daily
2. Bot randomly selects a content type (RSS, template, or AI)
3. Generates appropriate content
4. Posts to Twitter via API
5. Logs the result

## License

MIT
