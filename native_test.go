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

// TestResponsesStreamToolCall verifies the responses stream surfaces function
// call events: the item announcement (name/call_id), the arguments deltas and
// the final full arguments.
func TestResponsesStreamToolCall(t *testing.T) {
	events := []string{
		`event: response.output_item.added`,
		`data: {"type":"response.output_item.added","output_index":0,` +
			`"item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"get_weather","arguments":""}}`,
		``,
		`event: response.function_call_arguments.delta`,
		`data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","delta":"{\"city\":"}`,
		``,
		`event: response.function_call_arguments.delta`,
		`data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","delta":"\"Kyiv\"}"}`,
		``,
		`event: response.function_call_arguments.done`,
		`data: {"type":"response.function_call_arguments.done","item_id":"fc_1","arguments":"{\"city\":\"Kyiv\"}"}`,
		``,
		`event: response.completed`,
		`data: {"type":"response.completed","response":{"id":"r1"}}`,
		``,
	}
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		for _, line := range events {
			io.WriteString(w, line+"\n")
		}
	})
	defer done()

	var name, args string
	for ev, err := range c.ResponsesStream(context.Background(), &ResponsesRequest{
		Model: "m", Input: "weather?",
	}) {
		if err != nil {
			t.Fatal(err)
		}
		switch ev.Type {
		case "response.output_item.added":
			if ev.Item != nil {
				name = ev.Item.Name
			}
		case "response.function_call_arguments.delta":
			args += ev.Delta
		case "response.function_call_arguments.done":
			if ev.Arguments != args {
				t.Errorf("done arguments %q != accumulated %q", ev.Arguments, args)
			}
		}
	}
	if name != "get_weather" {
		t.Errorf("name = %q", name)
	}
	if args != `{"city":"Kyiv"}` {
		t.Errorf("args = %q", args)
	}
}
