package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/GowthamU7/ai-code-reviewer/github"
	"github.com/GowthamU7/ai-code-reviewer/groq"
	"github.com/GowthamU7/ai-code-reviewer/parser"
	"github.com/GowthamU7/ai-code-reviewer/store"
)

// PullRequestEvent is the shape of the JSON GitHub sends
// when a pull request is opened, updated, or closed
type PullRequestEvent struct {
	Action      string      `json:"action"`
	PullRequest PullRequest `json:"pull_request"`
	Repository  Repository  `json:"repository"`
}

type PullRequest struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	DiffURL string `json:"diff_url"`
	Head    struct {
		SHA string `json:"sha"`
	} `json:"head"`
}

type Repository struct {
	FullName string `json:"full_name"` // e.g. "gowtham/my-repo"
}

// Webhook is the HTTP handler for POST /webhook
func Webhook(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the raw body — we need it for HMAC verification
	// We must read it before anything else touches r.Body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading body: %v", err)
		http.Error(w, "could not read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Verify the signature before doing anything else
	// If this fails, someone is sending us fake webhooks
	if !verifySignature(body, r.Header.Get("X-Hub-Signature-256")) {
		log.Println("Invalid webhook signature — rejected")
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	// What type of event is this? We only care about pull_request
	eventType := r.Header.Get("X-GitHub-Event")
	log.Printf("Received event: %s", eventType)

	// Respond 200 immediately — GitHub expects a fast response
	// We do the actual work after this
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "accepted")

	// Only process pull_request events
	if eventType != "pull_request" {
		log.Printf("Ignoring event type: %s", eventType)
		return
	}

	// Parse the JSON body into our struct
	var event PullRequestEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("Error parsing webhook payload: %v", err)
		return
	}

	// Only act when a PR is opened or new commits are pushed to it
	if event.Action != "opened" && event.Action != "synchronize" {
		log.Printf("Ignoring PR action: %s", event.Action)
		return
	}

	log.Printf("PR #%d opened in %s: %s",
		event.PullRequest.Number,
		event.Repository.FullName,
		event.PullRequest.Title,
	)

	// Process in background so we don't block the HTTP response
	// This is a goroutine — Go's lightweight thread
	go processPullRequest(event)
}

// processPullRequest is where the real work happens
// It runs in a goroutine so the HTTP handler can return immediately
// SetDB sets the database for the handler package
var DB *store.DB

func processPullRequest(event PullRequestEvent) {
	log.Printf("Processing PR #%d in %s...",
		event.PullRequest.Number,
		event.Repository.FullName,
	)

	ghClient := github.New()
	rawDiff, err := ghClient.FetchDiff(event.PullRequest.DiffURL)
	if err != nil {
		log.Printf("Error fetching diff: %v", err)
		return
	}

	files := parser.ParseDiff(rawDiff)
	if len(files) == 0 {
		log.Println("No reviewable files found in diff")
		return
	}

	log.Printf("Reviewing %d files...", len(files))

	groqClient := groq.New()
	var fullReview strings.Builder

	fullReview.WriteString(fmt.Sprintf(
		"## AI Code Review for PR #%d\n\n",
		event.PullRequest.Number,
	))

	for _, file := range files {
		log.Printf("Reviewing %s...", file.Filename)

		review, err := groqClient.ReviewFile(
			file.Filename,
			file.Language,
			file.Content,
		)
		if err != nil {
			log.Printf("Error reviewing %s: %v", file.Filename, err)
			continue
		}

		fullReview.WriteString(fmt.Sprintf(
			"### `%s`\n\n%s\n\n---\n\n",
			file.Filename, review,
		))

		// Save each file review to the database
		if DB != nil {
			if err := DB.SaveReview(store.Review{
				Repo:       event.Repository.FullName,
				PRNumber:   event.PullRequest.Number,
				PRTitle:    event.PullRequest.Title,
				Filename:   file.Filename,
				Language:   file.Language,
				ReviewText: review,
			}); err != nil {
				log.Printf("Error saving review: %v", err)
			}
		}
	}

	// Post full review to GitHub
	if fullReview.Len() > 0 {
		err := ghClient.PostReview(
			event.Repository.FullName,
			event.PullRequest.Number,
			fullReview.String(),
		)
		if err != nil {
			log.Printf("Error posting review: %v", err)
			return
		}
		log.Printf("Review posted to PR #%d", event.PullRequest.Number)
	}
}

// verifySignature checks that the request genuinely came from GitHub
// GitHub computes HMAC-SHA256 of the body using your webhook secret
// We recompute it and compare — if they match, it's real
func verifySignature(body []byte, signatureHeader string) bool {
	secret := os.Getenv("GITHUB_WEBHOOK_SECRET")
	if secret == "" {
		log.Println("WARNING: GITHUB_WEBHOOK_SECRET not set — skipping verification")
		return true // allow through in dev if secret not configured
	}

	// GitHub sends the signature as "sha256=<hex>"
	// We need to strip the "sha256=" prefix before comparing
	if !strings.HasPrefix(signatureHeader, "sha256=") {
		return false
	}
	receivedSig := strings.TrimPrefix(signatureHeader, "sha256=")

	// Compute our own HMAC using the same secret
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	// Use hmac.Equal — this is constant-time comparison
	// Regular == comparison is vulnerable to timing attacks
	return hmac.Equal([]byte(receivedSig), []byte(expectedSig))
}
