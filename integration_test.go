//go:build integration

// Integration smoke tests hit the live OpenAI API. They are excluded from the
// normal build and run only with the "integration" tag and a real key:
//
//	OPENAI_API_KEY=sk-... go test -tags integration -run Integration ./...
package openai_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/goloop/ai"
	"github.com/goloop/openai"
)

func integrationClient(t *testing.T) *openai.Client {
	t.Helper()
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		t.Skip("set OPENAI_API_KEY to run integration tests")
	}
	return openai.New(key)
}

func TestIntegrationGenerate(t *testing.T) {
	c := integrationClient(t)
	resp, err := c.Generate(context.Background(), &ai.Request{
		Model:     openai.ModelGPT4oMini,
		MaxTokens: 16,
		Messages:  []ai.Message{ai.UserText("Reply with exactly one word: pong")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text() == "" {
		t.Fatal("empty text")
	}
	t.Logf("generate: %q (in=%d out=%d)", resp.Text(), resp.Usage.InputTokens, resp.Usage.OutputTokens)
}

func TestIntegrationStream(t *testing.T) {
	c := integrationClient(t)
	var text string
	var done bool
	var usage *ai.Usage
	for chunk, err := range c.Stream(context.Background(), &ai.Request{
		Model:     openai.ModelGPT4oMini,
		MaxTokens: 32,
		Messages:  []ai.Message{ai.UserText("Count from 1 to 5.")},
	}) {
		if err != nil {
			t.Fatal(err)
		}
		text += chunk.Text
		if chunk.Done {
			done, usage = true, chunk.Usage
		}
	}
	if text == "" || !done {
		t.Fatalf("text=%q done=%v", text, done)
	}
	t.Logf("stream: %q done=%v usage=%+v", text, done, usage)
}

func TestIntegrationTools(t *testing.T) {
	c := integrationClient(t)
	resp, err := c.Generate(context.Background(), &ai.Request{
		Model:     openai.ModelGPT4oMini,
		MaxTokens: 128,
		Messages:  []ai.Message{ai.UserText("What is the weather in Kyiv? Use the tool.")},
		Tools: []ai.Tool{{
			Name:        "get_weather",
			Description: "Get the current weather for a city.",
			Schema:      json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	// The model may answer in text or call the tool; both are well-formed. Log
	// what came back so a wire-format mismatch shows up.
	if resp.Text() == "" && len(resp.ToolCalls()) == 0 {
		t.Fatal("neither text nor tool call")
	}
	t.Logf("tools: text=%q calls=%d", resp.Text(), len(resp.ToolCalls()))
}

func TestIntegrationResponsesStream(t *testing.T) {
	c := integrationClient(t)
	var text string
	var completed bool
	for ev, err := range c.ResponsesStream(context.Background(), &openai.ResponsesRequest{
		Model: openai.ModelGPT4oMini,
		Input: "Say hello in one word.",
	}) {
		if err != nil {
			t.Fatal(err)
		}
		switch ev.Type {
		case "response.output_text.delta":
			text += ev.Delta
		case "response.completed":
			completed = true
		}
	}
	if text == "" || !completed {
		t.Fatalf("text=%q completed=%v", text, completed)
	}
	t.Logf("responses stream: %q", text)
}
