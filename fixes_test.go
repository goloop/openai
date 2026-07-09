package openai

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/goloop/ai"
)

// BUG-01: tool calls must be flushed even when the stream ends without a
// finish_reason of "tool_calls".
func TestStreamToolCallWithoutFinishReason(t *testing.T) {
	events := []string{
		`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1",` +
			`"function":{"name":"lookup","arguments":"{\"q\":42}"}}]}}]}`, ``,
		`data: [DONE]`, ``,
	}
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		for _, line := range events {
			io.WriteString(w, line+"\n")
		}
	})
	defer done()

	var call *ai.ToolUse
	for chunk, err := range c.Stream(context.Background(), &ai.Request{
		Model: "m", Messages: []ai.Message{ai.UserText("hi")},
	}) {
		if err != nil {
			t.Fatal(err)
		}
		if chunk.ToolCall != nil {
			call = chunk.ToolCall
		}
	}
	if call == nil || call.Name != "lookup" || string(call.Input) != `{"q":42}` {
		t.Fatalf("tool call lost: %+v", call)
	}
}

// BUG-02: native methods must not mutate the caller's ChatRequest.
func TestChatRequestNotMutated(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"model":"m","choices":[{"index":0,`+
			`"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)
	})
	defer done()

	req := &ChatRequest{Model: "m", Messages: []ChatMessage{{Role: "user", Content: "hi"}}}
	if _, err := c.ChatCompletion(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if req.Stream {
		t.Error("ChatCompletion mutated req.Stream")
	}
	if req.StreamOptions != nil {
		t.Error("req.StreamOptions unexpectedly set")
	}
}

// BUG-03: content returned as an array of parts must not be dropped.
func TestGenerateContentPartsResponse(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"model":"m","choices":[{"index":0,"message":{"role":"assistant",`+
			`"content":[{"type":"text","text":"Hello "},{"type":"text","text":"world"}]},`+
			`"finish_reason":"stop"}]}`)
	})
	defer done()

	resp, err := c.Generate(context.Background(), &ai.Request{
		Model: "m", Messages: []ai.Message{ai.UserText("hi")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text() != "Hello world" {
		t.Errorf("text = %q, want %q", resp.Text(), "Hello world")
	}
}

// BUG-04: a numeric "code" must not break error parsing.
func TestParseErrorNumericCode(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error":{"message":"bad","type":"invalid_request_error","code":429}}`)
	})
	defer done()

	_, err := c.Generate(context.Background(), &ai.Request{
		Model: "m", Messages: []ai.Message{ai.UserText("hi")},
	})
	var apiErr *ai.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v", err)
	}
	if apiErr.Message != "bad" {
		t.Errorf("message lost: %q", apiErr.Message)
	}
	if apiErr.Code != "429" {
		t.Errorf("code = %q, want \"429\"", apiErr.Code)
	}
}

func TestContentText(t *testing.T) {
	if got := contentText("hi"); got != "hi" {
		t.Errorf("string: %q", got)
	}
	arr := []any{
		map[string]any{"type": "text", "text": "a"},
		map[string]any{"type": "text", "text": "b"},
	}
	if got := contentText(arr); got != "ab" {
		t.Errorf("array: %q", got)
	}
	if got := contentText(nil); got != "" {
		t.Errorf("nil: %q", got)
	}
}
