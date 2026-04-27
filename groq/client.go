package groq

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

const (
	groqAPIURL = "https://api.groq.com/openai/v1/chat/completions"
	model      = "llama-3.3-70b-versatile"
)

// Client handles communication with the Groq API
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// New creates a Groq client using the API key from environment
func New() *Client {
	return &Client{
		apiKey: os.Getenv("GROQ_API_KEY"),
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Message is a single message in the conversation
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// request is the body we send to Groq
type request struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
}

// response is what Groq sends back
type response struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// ReviewFile sends a single file diff to Groq and returns a review
func (c *Client) ReviewFile(filename, language, diff string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("GROQ_API_KEY not set")
	}

	// Build the prompt
	// The system message sets the AI's role and behavior
	// The user message contains the actual code to review
	messages := []Message{
		{
			Role: "system",
			Content: `You are an expert code reviewer. Your job is to review git diffs and provide 
clear, actionable feedback. Focus on:
- Bugs and logic errors
- Security vulnerabilities  
- Performance issues
- Code clarity and maintainability
- Missing error handling

Format your response as a concise bullet-point list.
Be specific — reference line numbers and variable names where possible.
If the code looks good, say so briefly. Do not invent problems.
Keep your review under 300 words.`,
		},
		{
			Role: "user",
			Content: fmt.Sprintf(
				"Please review this %s code change in file `%s`:\n\n```diff\n%s\n```",
				language, filename, diff,
			),
		},
	}

	// Build request body
	reqBody := request{
		Model:       model,
		Messages:    messages,
		Temperature: 0.3, // lower = more focused, less creative
		MaxTokens:   500,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest(http.MethodPost, groqAPIURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	// Send it
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling Groq API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	// Parse response
	var groqResp response
	if err := json.Unmarshal(respBody, &groqResp); err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}

	// Check for API-level errors
	if groqResp.Error != nil {
		return "", fmt.Errorf("Groq API error: %s", groqResp.Error.Message)
	}

	if len(groqResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in Groq response")
	}

	review := groqResp.Choices[0].Message.Content
	log.Printf("Got review for %s (%d chars)", filename, len(review))
	return review, nil
}
