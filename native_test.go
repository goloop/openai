package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestResponsesStream(t *testing.T) {
	events := []string{
		`event: response.output_text.delta`,
		`data: {"type":"response.output_text.delta","delta":"Hel"}`,
		``,
		`event: response.output_text.delta`,
		`data: {"type":"response.output_text.delta","delta":"lo"}`,
		``,
		`event: response.completed`,
		`data: {"type":"response.completed","response":{"id":"r1","model":"m",` +
			`"status":"completed","usage":{"input_tokens":4,"output_tokens":2}}}`,
		``,
	}
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var req ResponsesRequest
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatal(err)
		}
		if !req.Stream {
			t.Errorf("stream not set: %s", body)
		}
		for _, line := range events {
			io.WriteString(w, line+"\n")
		}
	})
	defer done()

	var text strings.Builder
	var out int
	for ev, err := range c.ResponsesStream(context.Background(), &ResponsesRequest{
		Model: "m", Input: "hi",
	}) {
		if err != nil {
			t.Fatal(err)
		}
		switch ev.Type {
		case "response.output_text.delta":
			text.WriteString(ev.Delta)
		case "response.completed":
			if ev.Response != nil {
				out = ev.Response.Usage.OutputTokens
			}
		}
	}
	if text.String() != "Hello" {
		t.Errorf("text = %q", text.String())
	}
	if out != 2 {
		t.Errorf("output tokens = %d", out)
	}
}

func TestResponsesStreamError(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error":{"message":"bad model","type":"invalid_request_error"}}`)
	})
	defer done()

	var got error
	for _, err := range c.ResponsesStream(context.Background(), &ResponsesRequest{
		Model: "m", Input: "hi",
	}) {
		got = err
	}
	if got == nil || !strings.Contains(got.Error(), "bad model") {
		t.Fatalf("err = %v", got)
	}
}

// TestResponsesRequestNotMutated verifies the caller's request is untouched by
// the streaming call setting Stream.
func TestResponsesRequestNotMutated(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "event: response.completed\n"+
			`data: {"type":"response.completed","response":{"id":"r"}}`+"\n\n")
	})
	defer done()

	req := &ResponsesRequest{Model: "m", Input: "hi"}
	for range c.ResponsesStream(context.Background(), req) {
	}
	if req.Stream {
		t.Error("caller request was mutated: Stream = true")
	}
}
