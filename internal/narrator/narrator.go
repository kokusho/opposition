// Package narrator has a language model write a prep report up in prose,
// through any server that speaks the OpenAI chat completions protocol.
package narrator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// instructions is what the model is told. Language models are weak at chess
// and fluent at sounding sure, so the rules keep it to restating what the
// database, the engine and the prediction model found.
const instructions = `You write a short preparation briefing for a chess player about one opponent.

You are given facts gathered from the opponent's games, a chess engine and a model that predicts human moves. Rules:
- Use only the facts given. Do not add moves, variations, evaluations, statistics or plans of your own.
- Copy moves and numbers exactly as they appear. Never complete or extend a line.
- Name an opening only if you are certain of it from the moves shown; otherwise leave it unnamed.
- Say how many games a claim rests on. With fewer than five games, say that it is thin evidence.
- Percentages under "Traps" are a model's estimate for a rating, not this opponent's record. Say so when you use them.
- If a section says it was not analysed or that nothing was found, say that plainly instead of filling the gap.

Write plain text without markdown, in this order:
1. Two or three sentences on who the opponent is as a player with this colour.
2. "What to play": the one to three most promising concrete choices, best first, each with the moves and the reason from the facts.
3. "What to expect": the main lines they are likely to go for.
4. "Caveats": what the evidence does not cover.
Keep it under 300 words.`

const (
	temperature  = 0.3
	requestLimit = 2 * time.Minute
	errorSnippet = 300
)

// Client talks to an OpenAI-compatible chat completions endpoint.
type Client struct {
	// BaseURL ends before "/chat/completions", e.g. "https://api.openai.com/v1"
	// or "http://localhost:11434/v1".
	BaseURL string
	Model   string
	// APIKey is sent as a bearer token; empty sends none.
	APIKey string
	// HTTP is the client to use; nil means a default one with a timeout.
	HTTP *http.Client
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Narrate asks the model to write the facts up. The facts are sent as they
// are; the rules for writing are this package's.
func (c Client) Narrate(ctx context.Context, facts string) (string, error) {
	payload, err := json.Marshal(struct {
		Model       string    `json:"model"`
		Messages    []message `json:"messages"`
		Temperature float64   `json:"temperature"`
	}{
		Model:       c.Model,
		Messages:    []message{{Role: "system", Content: instructions}, {Role: "user", Content: facts}},
		Temperature: temperature,
	})
	if err != nil {
		return "", err
	}

	url := strings.TrimRight(c.BaseURL, "/") + "/chat/completions"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		request.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: requestLimit}
	}
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("asking the language model: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("reading the language model's answer: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		snippet := strings.TrimSpace(string(body))
		if len(snippet) > errorSnippet {
			snippet = snippet[:errorSnippet]
		}
		return "", fmt.Errorf("language model answered %d: %s", response.StatusCode, snippet)
	}

	var answer struct {
		Choices []struct {
			Message message `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		return "", fmt.Errorf("language model sent an unreadable answer: %w", err)
	}
	if len(answer.Choices) == 0 || strings.TrimSpace(answer.Choices[0].Message.Content) == "" {
		return "", errors.New("language model sent an answer without text")
	}
	return strings.TrimSpace(answer.Choices[0].Message.Content), nil
}
