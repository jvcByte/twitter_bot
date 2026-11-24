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

The bot posts through a saved browser session (cookies) — X blocks automated password logins far more often, and accounts with 2FA cannot use one at all.

1. Open [x.com](https://x.com) in Chrome or Firefox and make sure you're logged in
2. Install the **Cookie-Editor** extension: [cookie-editor.com](https://cookie-editor.com)
3. Click the Cookie-Editor icon in your browser toolbar
4. Click **Export** → **Export as JSON**
5. Copy everything — that's your cookies. You'll paste it in Step 4.

---

### Step 3 — Get free AI keys

The bot uses an LLM to write posts and comments. All three options have a free tier — no credit card needed. You need at least one.

- **Groq** (primary): [console.groq.com/keys](https://console.groq.com/keys) — sign up free, create a key, copy it (~1,000 requests/day)
- **Gemini** (fallback): [aistudio.google.com/app/apikey](https://aistudio.google.com/app/apikey) — sign in with Google, click "Create API key", copy it
- **OpenRouter** (fallback): [openrouter.ai/keys](https://openrouter.ai/keys) — sign up free, create a key, copy it

---

### Step 4 — Add your secrets to GitHub

In your forked repo: **Settings → Secrets and variables → Actions → New repository secret**

Each workflow reads the secret names written in its own `env:` block. The workflows shipped in this repo use account-prefixed names (`SIMPLE_CHEENA_*`, `THE_ONLY_TECH_DUDE_*`) — create the secrets with those prefixes, or rename the `env:` entries if you run a single account. The values are the same:

| Name | What to put in it |
|---|---|
| `TWITTER_USERNAME` | Your X username (no @ symbol) |
| `TWITTER_COOKIES` | The cookie JSON from Step 2 |
| `GROQ_API_KEY` | Your Groq key from Step 3 (primary) |
| `GEMINI_API_KEY` | Your Gemini key from Step 3 (fallback) |
| `OPENROUTER_API_KEY` | Your OpenRouter key from Step 3 (fallback) |

---

### Running on multiple accounts

Each account gets its own workflow file. Duplicate any of the `post_*.yml` files under `.github/workflows/` and give it a name that matches the account, e.g. `post_mySecondAccount.yml`.

In that file, set the `env:` block to use its own set of secrets:

```yaml
env:
  TWITTER_USERNAME: ${{ secrets.MY_SECOND_ACCOUNT_TWITTER_USERNAME }}
  TWITTER_COOKIES:  ${{ secrets.MY_SECOND_ACCOUNT_TWITTER_COOKIES }}
```

Then add those two secrets in GitHub (same place as Step 4) with the matching names. Each workflow runs independently on its own schedule — no other changes needed.

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

**Bot says "session invalid"** — Your cookies expired. Follow the "Updating Your Cookies" steps above.

**Bot isn't posting anything** — Check the Actions tab, click on a recent run, and look at the logs. The most common cause is missing secrets or cookies that need refreshing.

**Posts keep repeating** — Make sure Step 6 (workflow permissions) is done. Without it the bot can't save which articles it already used.

**I only want news posts / only want engagement** — Change the `POST_MODE` variable (Step 5) to `news` or `engage`.

---

## License

[MIT](./LICENSE)
