package narrator_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"opposition/internal/narrator"
)

type chatRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

// fakeLLM answers chat completion requests and records the last one.
type fakeLLM struct {
	*httptest.Server
	path, authorization string
	request             chatRequest
	status              int
	body                string
}

func newFakeLLM(t *testing.T) *fakeLLM {
	t.Helper()
	fake := &fakeLLM{status: http.StatusOK, body: `{"choices":[{"message":{"role":"assistant","content":"Play 2.c3."}}]}`}
	fake.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fake.path, fake.authorization = r.Method+" "+r.URL.Path, r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &fake.request)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(fake.status)
		_, _ = io.WriteString(w, fake.body)
	}))
	t.Cleanup(fake.Close)
	return fake
}

func TestNarrateSendsTheFactsAsAChatCompletionAndReturnsTheAnswer(t *testing.T) {
	llm := newFakeLLM(t)
	client := narrator.Client{BaseURL: llm.URL + "/v1", Model: "some-model", APIKey: "secret"}

	text, err := client.Narrate(context.Background(), "Opponent: Petrov, Ivan")

	if err != nil || text != "Play 2.c3." {
		t.Fatalf("text = %q, err = %v", text, err)
	}
	if llm.path != "POST /v1/chat/completions" || llm.authorization != "Bearer secret" {
		t.Errorf("request was %q with authorization %q", llm.path, llm.authorization)
	}
	if llm.request.Model != "some-model" || len(llm.request.Messages) != 2 {
		t.Fatalf("request = %+v, want the model and two messages", llm.request)
	}
	system, user := llm.request.Messages[0], llm.request.Messages[1]
	if system.Role != "system" || !strings.Contains(system.Content, "only the facts") {
		t.Errorf("system message = %+v, want the rule to use only the facts", system)
	}
	if user.Role != "user" || user.Content != "Opponent: Petrov, Ivan" {
		t.Errorf("user message = %+v, want the facts unchanged", user)
	}
}

func TestNarrateSendsNoAuthorizationWithoutAKeyAndToleratesATrailingSlash(t *testing.T) {
	llm := newFakeLLM(t)
	client := narrator.Client{BaseURL: llm.URL + "/v1/", Model: "local"}

	if _, err := client.Narrate(context.Background(), "facts"); err != nil {
		t.Fatal(err)
	}
	if llm.path != "POST /v1/chat/completions" || llm.authorization != "" {
		t.Errorf("request was %q with authorization %q", llm.path, llm.authorization)
	}
}

func TestNarrateReportsARefusalFromTheServer(t *testing.T) {
	llm := newFakeLLM(t)
	llm.status, llm.body = http.StatusUnauthorized, `{"error":{"message":"bad key"}}`
	client := narrator.Client{BaseURL: llm.URL, Model: "m"}

	_, err := client.Narrate(context.Background(), "facts")

	if err == nil || !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "bad key") {
		t.Errorf("err = %v, want the status and the server's message", err)
	}
}

func TestNarrateFailsOnAnAnswerWithoutText(t *testing.T) {
	for _, body := range []string{`{"choices":[]}`, `{"choices":[{"message":{"content":"  "}}]}`, `not json`} {
		llm := newFakeLLM(t)
		llm.body = body
		client := narrator.Client{BaseURL: llm.URL, Model: "m"}

		if text, err := client.Narrate(context.Background(), "facts"); err == nil {
			t.Errorf("body %q: got %q, want an error", body, text)
		}
	}
}
