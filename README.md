# Twitter AI Bot

An automated X (Twitter) bot that posts news, tips, opinions, and engages with people — all on autopilot. No server needed. Runs free on GitHub.

---

## What It Does

Runs on a schedule and does four things:

- **Posts news** — finds fresh articles and tweets about them
- **Posts your own content** — tips, opinions, and takes based on real articles (nothing invented)
- **Posts memes & polls** — funny or engaging tech posts
- **Engages with people** — likes, comments, and reposts relevant posts from others

---

## Setup (Step by Step)

### Step 1 — Fork this repo

Go to the top of this page and click **Fork**. This creates your own copy.

---

### Step 2 — Log the bot into your X account

The bot needs a way to post as you. You have two options:

**Option A — Username & Password (easiest, works on any device)**

Just add your username and password in Step 4. The bot will log in automatically. Done.

> If X ever blocks the login, switch to Option B.

**Option B — Session cookies (more reliable, requires a laptop/desktop)**

Cookies are a saved login session — they're harder for X to block than a password login.

1. Open [x.com](https://x.com) in Chrome or Firefox on your **laptop or desktop** and make sure you're logged in
2. Install the **Cookie-Editor** extension: [cookie-editor.com](https://cookie-editor.com)
3. Click the Cookie-Editor icon in your browser toolbar
4. Click **Export** → **Export as JSON**
5. Copy everything — that's your cookies. You'll paste it in Step 4.

> If you only have a phone, use Option A and skip the cookies.

---

### Step 3 — Get free AI keys

The bot uses free AI services to write tweets. Get keys from:

- **Gemini** (primary): [aistudio.google.com/app/apikey](https://aistudio.google.com/app/apikey) — sign in with Google, click "Create API key", copy it
- **OpenRouter** (backup): [openrouter.ai/keys](https://openrouter.ai/keys) — sign up free, create a key, copy it

---

### Step 4 — Add your secrets to GitHub

In your forked repo: **Settings → Secrets and variables → Actions → New repository secret**

Add each of these:

| Name | What to put in it |
|---|---|
| `TWITTER_USERNAME` | Your X username (no @ symbol) |
| `TWITTER_PASSWORD` | Your X password |
| `TWITTER_COOKIES` | The cookie JSON from Step 2 Option B — skip if using Option A |
| `GEMINI_API_KEY` | Your Gemini key from Step 3 |
| `OPENROUTER_API_KEY` | Your OpenRouter key from Step 3 |

**Second account (optional)** — if you want to run the bot on a second X account, also add:
`TECH_TWITTER_USERNAME`, `TECH_TWITTER_PASSWORD`, `TECH_TWITTER_COOKIES`

---

### Step 5 — Set your posting mode

In your forked repo: **Settings → Secrets and variables → Actions → Variables tab → New repository variable**

| Name | Value | What it means |
|---|---|---|
| `POST_MODE` | `mixed` | Rotates between news, tips, memes, and engagement |
| `MAX_ARTICLE_AGE_HOURS` | `7` | Only post articles from the last 7 hours |
| `MAX_TWEETS_PER_RUN` | `5` | Post up to 5 tweets per run |

> You can also set `POST_MODE` to `news`, `creator`, `meme`, or `engage` if you only want one type.

---

### Step 6 — Allow the bot to save its progress

Go to **Settings → Actions → General**, scroll to **Workflow permissions**, select **Read and write permissions**, and click Save.

This lets the bot remember which articles it already posted so it doesn't repeat itself.

---

### Step 7 — Turn on the workflows

Go to the **Actions** tab in your repo. If you see a banner saying workflows are disabled, click **Enable**.

The bot will now run automatically:
- Every 6 hours to post content
- Every 30 minutes to engage with other people's posts

**To test it right now:** Actions → pick a workflow → click **Run workflow**

---

## Updating Your Cookies

Cookies expire after 30–90 days. When the bot stops working, just:

1. Go to x.com, log in
2. Export cookies again with Cookie-Editor
3. Go to your repo → **Settings → Secrets → TWITTER_COOKIES** → click Update → paste new cookies

---

## Troubleshooting

**Bot says "session invalid"** — If you're using cookies (Option B), they expired. Follow the "Updating Your Cookies" steps above. If you're using username/password (Option A), X may have temporarily blocked automated logins — wait a few hours and try again, or switch to cookies.

**Bot isn't posting anything** — Check the Actions tab, click on a recent run, and look at the logs. The most common cause is missing secrets or cookies that need refreshing.

**Posts keep repeating** — Make sure Step 6 (workflow permissions) is done. Without it the bot can't save which articles it already used.

**I only want news posts / only want engagement** — Change the `POST_MODE` variable (Step 5) to `news` or `engage`.

---

## License

[MIT](./LICENSE)
