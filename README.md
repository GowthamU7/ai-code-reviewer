# AI Code Reviewer

> Automated pull request reviews powered by Go, Groq (LLaMA 3.3), and GitHub Webhooks — with a React dashboard to track review history.

[![Go](https://img.shields.io/badge/Go-1.24-00ADD8?style=flat&logo=go)](https://golang.org)
[![Groq](https://img.shields.io/badge/LLM-Groq%20LLaMA%203.3-F55036?style=flat)](https://groq.com)
[![Railway](https://img.shields.io/badge/Deployed-Railway-0B0D0E?style=flat&logo=railway)](https://railway.app)
[![React](https://img.shields.io/badge/Dashboard-React%20+%20Vite-61DAFB?style=flat&logo=react)](https://vitejs.dev)
[![PostgreSQL](https://img.shields.io/badge/Database-PostgreSQL-336791?style=flat&logo=postgresql)](https://postgresql.org)

---

## What it does

Every time a pull request is opened or updated on a connected GitHub repository, this system automatically:

1. Receives the webhook event from GitHub
2. Verifies the HMAC-SHA256 signature to confirm the request is genuine
3. Fetches the raw unified diff from GitHub's API
4. Parses the diff into per-file chunks, filtering out generated files, lock files, and vendor directories
5. Sends each file's diff to Groq's LLaMA 3.3 70B model with a structured code review prompt
6. Posts the AI-generated review back to the pull request as a GitHub review comment
7. Stores the review in PostgreSQL for historical tracking
8. Displays all reviews in a React dashboard

**The entire pipeline — from webhook received to review posted — completes in under 5 seconds.**

---

## Live demo

| Service | URL |
|---------|-----|
| Backend API | https://your-app.up.railway.app/health |
| Reviews API | https://your-app.up.railway.app/reviews |
| Dashboard | https://ai-code-reviewer-dashboard-chi.vercel.app/ |

---

## Architecture

```
GitHub PR opened
       │
       ▼
POST /webhook (Go server on Railway)
       │
       ├── Verify HMAC-SHA256 signature
       ├── Parse X-GitHub-Event header
       └── Spawn goroutine (respond 200 immediately)
              │
              ├── Fetch diff from GitHub API
              ├── Parse diff into per-file chunks
              │      └── Filter: skip vendor/, lock files, generated code
              │
              ├── For each file:
              │      ├── Build prompt with file + language context
              │      ├── Call Groq API (LLaMA 3.3 70B)
              │      └── Store review in PostgreSQL
              │
              └── Post full review to GitHub PR via Reviews API
```

---

## Tech stack

| Layer | Technology | Why |
|-------|-----------|-----|
| Backend | Go 1.24 | Concurrency model fits perfectly — goroutines handle async processing without blocking the HTTP response |
| LLM | Groq + LLaMA 3.3 70B | Fastest inference available — reviews complete in 1-2 seconds per file |
| Webhook security | HMAC-SHA256 | Constant-time comparison prevents timing attacks |
| Database | PostgreSQL | Stores review history; auto-provisioned via Railway |
| Frontend | React + Vite + TypeScript | Dashboard for review history with per-file drill-down |
| Deployment | Railway (backend) + Vercel (frontend) | Zero-config deployment from GitHub push |
| CI/CD | GitHub Actions | Runs on every push to main |

---

## Project structure

```
ai-code-reviewer/
├── main.go                 # HTTP server, routing, DB init
├── handler/
│   └── webhook.go          # GitHub webhook receiver + HMAC verification
├── github/
│   └── client.go           # GitHub API client (fetch diff, post review)
├── groq/
│   └── client.go           # Groq API client (LLM inference)
├── parser/
│   └── diff.go             # Git diff parser + file filtering
├── store/
│   └── db.go               # PostgreSQL client + migrations
├── dashboard/              # React + Vite frontend
│   ├── src/
│   │   └── App.tsx         # Review history dashboard
│   └── package.json
├── Dockerfile              # Multi-stage build (Go → Alpine)
└── .github/
    └── workflows/
        └── ci.yml          # GitHub Actions CI
```

---

## How to run locally

**Prerequisites:** Go 1.24+, Node 18+, PostgreSQL (optional)

**1. Clone and install:**

```bash
git clone https://github.com/GowthamU7/ai-code-reviewer.git
cd ai-code-reviewer
go mod download
```

**2. Configure environment:**

```bash
cp .env.example .env
```

Edit `.env`:

```env
GITHUB_WEBHOOK_SECRET=your-secret
GITHUB_TOKEN=ghp_yourtoken
GROQ_API_KEY=gsk_yourkey
DATABASE_URL=postgres://user:pass@localhost/reviewer
PORT=8080
```

**3. Run the Go server:**

```bash
go run main.go
```

**4. Expose locally with ngrok (for webhook testing):**

```bash
ngrok http 8080
```

Use the ngrok URL as your GitHub webhook payload URL.

**5. Run the dashboard:**

```bash
cd dashboard
npm install
npm run dev
```

Dashboard runs at `http://localhost:5173`.

---

## Deploying your own instance

**Backend (Railway):**

1. Fork this repo
2. Create a new Railway project → Deploy from GitHub
3. Add a PostgreSQL database (Railway auto-injects `DATABASE_URL`)
4. Set environment variables: `GITHUB_WEBHOOK_SECRET`, `GITHUB_TOKEN`, `GROQ_API_KEY`
5. Railway builds the Dockerfile and deploys automatically on every push

**Frontend (Vercel):**

```bash
cd dashboard
npx vercel --prod
```

Set `VITE_API_URL` to your Railway URL in Vercel's environment variables.

**GitHub webhook:**

In your target repo → Settings → Webhooks → Add webhook:
- Payload URL: `https://your-railway-url.up.railway.app/webhook`
- Content type: `application/json`
- Secret: your `GITHUB_WEBHOOK_SECRET`
- Events: Pull requests only

---

## Key engineering decisions

**Why Go over Python/Node for the backend?**
Go's goroutines make the async pattern trivial — respond `200 OK` to GitHub immediately, process the review in the background. No event loop complexity, no async/await chains. The binary compiles to ~15MB and starts in milliseconds.

**Why HMAC-SHA256 verification matters**
The webhook endpoint is public. Without signature verification, anyone who discovers the URL can trigger fake reviews or flood the system. The HMAC check uses `hmac.Equal` (constant-time comparison) to prevent timing attacks where an attacker could guess the signature byte by byte.

**Why per-file reviews instead of the whole diff?**
LLMs have context windows. A large PR touching 20 files could easily exceed the token limit if sent as one payload. Chunking per file also produces more focused, actionable feedback and lets us truncate oversized individual diffs gracefully.

**Why Groq over OpenAI?**
Inference speed. Groq's custom hardware (LPUs) runs LLaMA 3.3 70B at ~800 tokens/second — roughly 10x faster than OpenAI's GPT-4o for this use case. For a code review bot where latency directly affects developer experience, this matters.

---

## Example review output

```markdown
## AI Code Review for PR #42

### `src/auth/middleware.go`

- **Security issue (line 34):** JWT token is validated but the `exp` claim is never
  checked. An expired token will still pass authentication.
- **Missing error handling (line 67):** `json.Unmarshal` error is silently ignored.
  If the payload is malformed, the handler proceeds with a zero-value struct.
- **Performance (line 89):** Database query inside a loop. Consider batching the
  user lookups into a single query with `WHERE id = ANY($1)`.
- The overall structure is clean and the middleware chain is well-organised.

---

### `src/auth/middleware_test.go`

- Good coverage of the happy path.
- Missing test case for expired tokens — given the bug noted above, this would
  have caught it.
- Consider table-driven tests to reduce repetition across the 8 similar test functions.
```

---

## What I learned building this

- **Go's `net/http` standard library** is powerful enough for production webhook servers without a framework
- **HMAC-SHA256 verification** and why constant-time comparison (`hmac.Equal`) is critical for security
- **Goroutines** for decoupling HTTP response time from processing time
- **Prompt engineering** for code review — temperature 0.3 produces focused, deterministic reviews; higher values hallucinate problems
- **Multi-stage Docker builds** to keep production images small (15MB vs 800MB)
- **Railway + Vercel** for zero-friction deployment of Go backends and React frontends

---

## Future improvements

- [ ] Line-level comments using GitHub's pull request review comments API (requires parsing hunk headers)
- [ ] Configurable review rules per repository via a `.reviewer.yml` config file
- [ ] Review severity scoring (critical / warning / suggestion) with dashboard filtering
- [ ] Slack/Teams notification when a review is posted
- [ ] Support for GitLab and Bitbucket webhooks
- [ ] Rate limiting per repository to prevent abuse

---

## Author

**Gowtham Ullangula**
[LinkedIn](https://www.linkedin.com/in/gowthamm9/overlay/Project/737691022/treasury/?profileId=ACoAADhS2YQB552dW7mEsxlbNlUtpJ0D04OYFHQ) · [GitHub](https://github.com/GowthamU7)
