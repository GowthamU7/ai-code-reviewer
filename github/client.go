package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// Client handles all communication with the GitHub API
type Client struct {
	token      string
	httpClient *http.Client
}

// New creates a GitHub client using the token from environment
func New() *Client {
	return &Client{
		token: os.Getenv("GITHUB_TOKEN"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// FetchDiff downloads the raw unified diff for a pull request
// The diffURL looks like: https://github.com/owner/repo/pull/1.diff
func (c *Client) FetchDiff(diffURL string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, diffURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	// GitHub requires Accept header for diff format
	req.Header.Set("Accept", "application/vnd.github.diff")

	// Auth header — needed for private repos, also increases rate limit
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching diff: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub returned status %d for diff URL", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading diff body: %w", err)
	}

	log.Printf("Fetched diff: %d bytes", len(body))
	return string(body), nil
}

// reviewRequest is the body we send to GitHub's PR review API
type reviewRequest struct {
	Body  string `json:"body"`
	Event string `json:"event"`
}

// PostReview posts the full review as a PR comment on GitHub
// repo is "owner/repo", prNumber is the PR number, body is the review text
func (c *Client) PostReview(repo string, prNumber int, body string) error {
	if c.token == "" {
		return fmt.Errorf("GITHUB_TOKEN not set — cannot post review")
	}

	url := fmt.Sprintf(
		"https://api.github.com/repos/%s/pulls/%d/reviews",
		repo, prNumber,
	)

	payload := reviewRequest{
		Body:  body,
		Event: "COMMENT", // COMMENT = post without approving or requesting changes
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling review: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("posting review: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(respBody))
	}

	log.Printf("Posted review to %s PR #%d", repo, prNumber)
	return nil
}
