package openai

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/goloop/ai"
)

// A chat stream that ends without a [DONE] sentinel was truncated and must
// surface an error, not report Done with a cut-off result.
func TestStreamTruncatedNoDone(t *testing.T) {
	events := []string{
		`data: {"choices":[{"index":0,"delta":{"content":"Hel"}}]}`, ``,
		// connection ends here - no [DONE]
	}
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		for _, line := range events {
			io.WriteString(w, line+"\n")
		}
	})
	defer done()

	var gotErr error
	var doneSeen bool
	for chunk, err := range c.Stream(context.Background(), &ai.Request{
		Model: "m", Messages: []ai.Message{ai.UserText("hi")},
	}) {
		if err != nil {
			gotErr = err
			break
		}
		if chunk.Done {
			doneSeen = true
		}
	}
	if doneSeen {
		t.Error("truncated stream should not report Done")
	}
	if !errors.Is(gotErr, io.ErrUnexpectedEOF) {
		t.Errorf("err = %v, want ErrUnexpectedEOF", gotErr)
	}
}

// A streamed tool call whose accumulated arguments are not valid JSON must be
// reported as an error, not yielded as an unparseable Input.
func TestStreamInvalidToolArgs(t *testing.T) {
	events := []string{
		`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1",` +
			`"function":{"name":"lookup","arguments":"{not json"}}]},"finish_reason":"tool_calls"}]}`, ``,
		`data: [DONE]`, ``,
	}
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		for _, line := range events {
			io.WriteString(w, line+"\n")
		}
	})
	defer done()

	var gotErr error
	for chunk, err := range c.Stream(context.Background(), &ai.Request{
		Model: "m", Messages: []ai.Message{ai.UserText("hi")},
	}) {
		if err != nil {
			gotErr = err
			break
		}
		if chunk.ToolCall != nil {
			t.Error("invalid tool args must not yield a tool call")
		}
	}
	if gotErr == nil {
		t.Fatal("want error for invalid tool-call JSON, got nil")
	}
}

// The native ResponsesStream must surface a malformed SSE JSON payload as an
// error rather than silently skipping it.
func TestResponsesStreamMalformedJSON(t *testing.T) {
	events := []string{
		`event: x`, `data: {not valid json`, ``,
	}
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		for _, line := range events {
			io.WriteString(w, line+"\n")
		}
	})
	defer done()

	var gotErr error
	for _, err := range c.ResponsesStream(context.Background(), &ResponsesRequest{
		Model: "m", Input: "hi",
	}) {
		if err != nil {
			gotErr = err
			break
		}
	}
	if gotErr == nil {
		t.Fatal("want error for malformed SSE JSON, got nil")
	}
}

// A responses stream that ends without a terminal event was truncated.
func TestResponsesStreamTruncated(t *testing.T) {
	events := []string{
		`event: response.output_text.delta`,
		`data: {"type":"response.output_text.delta","delta":"Hel"}`, ``,
		// no response.completed
	}
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		for _, line := range events {
			io.WriteString(w, line+"\n")
		}
	})
	defer done()

	var gotErr error
	for _, err := range c.ResponsesStream(context.Background(), &ResponsesRequest{
		Model: "m", Input: "hi",
	}) {
		if err != nil {
			gotErr = err
			break
		}
	}
	if !errors.Is(gotErr, io.ErrUnexpectedEOF) {
		t.Errorf("err = %v, want ErrUnexpectedEOF", gotErr)
	}
}

// Public methods that take a request pointer must return an error, not panic,
// on a nil argument.
func TestNilRequestGuards(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server must not be reached for a nil request")
	})
	defer done()

	if _, err := c.ChatCompletion(context.Background(), nil); !errors.Is(err, ai.ErrNoRequest) {
		t.Errorf("ChatCompletion(nil) = %v", err)
	}
	if _, err := c.CreateResponse(context.Background(), nil); !errors.Is(err, ai.ErrNoRequest) {
		t.Errorf("CreateResponse(nil) = %v", err)
	}
	if _, err := c.Transcribe(context.Background(), nil); !errors.Is(err, ai.ErrNoRequest) {
		t.Errorf("Transcribe(nil) = %v", err)
	}
	if _, err := c.Translate(context.Background(), nil); !errors.Is(err, ai.ErrNoRequest) {
		t.Errorf("Translate(nil) = %v", err)
	}
	if _, err := c.Speech(context.Background(), nil); !errors.Is(err, ai.ErrNoRequest) {
		t.Errorf("Speech(nil) = %v", err)
	}
	if err := c.SpeechTo(context.Background(), nil, io.Discard); !errors.Is(err, ai.ErrNoRequest) {
		t.Errorf("SpeechTo(nil) = %v", err)
	}
	for _, err := range c.ChatCompletionStream(context.Background(), nil) {
		if !errors.Is(err, ai.ErrNoRequest) {
			t.Errorf("ChatCompletionStream(nil) = %v", err)
		}
		break
	}
	for _, err := range c.ResponsesStream(context.Background(), nil) {
		if !errors.Is(err, ai.ErrNoRequest) {
			t.Errorf("ResponsesStream(nil) = %v", err)
		}
		break
	}
}

// FileContentTo and SpeechTo stream a successful body straight to the writer.
func TestStreamToWriter(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/content") {
			io.WriteString(w, "file-bytes")
			return
		}
		io.WriteString(w, "audio-bytes")
	})
	defer done()

	var fb bytes.Buffer
	if err := c.FileContentTo(context.Background(), "file_1", &fb); err != nil {
		t.Fatal(err)
	}
	if fb.String() != "file-bytes" {
		t.Errorf("file = %q", fb.String())
	}
	var sb bytes.Buffer
	if err := c.SpeechTo(context.Background(),
		&SpeechRequest{Model: "tts", Input: "hi", Voice: "alloy"}, &sb); err != nil {
		t.Fatal(err)
	}
	if sb.String() != "audio-bytes" {
		t.Errorf("speech = %q", sb.String())
	}
}

// A JSON response body larger than the ceiling must error, not be truncated.
func TestResponseBodyCapped(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1<<20)
		for i := 0; i <= maxJSONBytes/len(buf)+1; i++ {
			if _, err := w.Write(buf); err != nil {
				return
			}
		}
	})
	defer done()

	if _, err := c.Models(context.Background()); err == nil {
		t.Fatal("want error for oversized response body, got nil")
	}
}

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
