package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestChatCompletionNative(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &req)
		if string(req.ResponseFormat) != `{"type":"json_object"}` {
			t.Errorf("response_format = %s", req.ResponseFormat)
		}
		io.WriteString(w, `{"model":"m","choices":[{"index":0,`+
			`"message":{"role":"assistant","content":"{}"},"finish_reason":"stop"}]}`)
	})
	defer done()

	resp, err := c.ChatCompletion(context.Background(), &ChatRequest{
		Model:          "m",
		Messages:       []ChatMessage{{Role: "user", Content: "hi"}},
		ResponseFormat: json.RawMessage(`{"type":"json_object"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Choices[0].Message.Content != "{}" {
		t.Errorf("content = %v", resp.Choices[0].Message.Content)
	}
}

func TestChatCompletionStreamNative(t *testing.T) {
	events := []string{
		`data: {"choices":[{"index":0,"delta":{"content":"a"}}]}`, ``,
		`data: {"choices":[{"index":0,"delta":{"content":"b"}}]}`, ``,
		`data: [DONE]`, ``,
	}
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		for _, line := range events {
			io.WriteString(w, line+"\n")
		}
	})
	defer done()

	var text strings.Builder
	for chunk, err := range c.ChatCompletionStream(context.Background(), &ChatRequest{
		Model: "m", Messages: []ChatMessage{{Role: "user", Content: "hi"}},
	}) {
		if err != nil {
			t.Fatal(err)
		}
		for _, ch := range chunk.Choices {
			text.WriteString(ch.Delta.Content)
		}
	}
	if text.String() != "ab" {
		t.Errorf("text = %q", text.String())
	}
}

func TestResponses(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/responses") {
			t.Errorf("path = %q", r.URL.Path)
		}
		io.WriteString(w, `{"id":"resp_1","model":"m","status":"completed",`+
			`"output":[{"type":"message","content":[{"type":"output_text","text":"hi there"}]}],`+
			`"usage":{"input_tokens":2,"output_tokens":3}}`)
	})
	defer done()

	resp, err := c.CreateResponse(context.Background(), &ResponsesRequest{
		Model: "m", Input: "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text() != "hi there" || resp.Usage.OutputTokens != 3 {
		t.Errorf("resp = %+v", resp)
	}
}

func TestTranslate(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/audio/translations") {
			t.Errorf("path = %q", r.URL.Path)
		}
		io.WriteString(w, `{"text":"hello"}`)
	})
	defer done()

	text, err := c.Translate(context.Background(), &TranscriptionRequest{
		Model: "whisper-1", File: []byte("x"), Filename: "a.mp3",
	})
	if err != nil || text != "hello" {
		t.Fatalf("translate: %v %q", err, text)
	}
}

func TestGetModel(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"id":"gpt-4o","object":"model"}`)
	})
	defer done()

	m, err := c.GetModel(context.Background(), "gpt-4o")
	if err != nil || m.ID != "gpt-4o" {
		t.Fatalf("model: %v %+v", err, m)
	}
}

func TestFileOps(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/files" && r.Method == http.MethodGet:
			io.WriteString(w, `{"data":[{"id":"file_1","filename":"a"}]}`)
		case strings.HasSuffix(r.URL.Path, "/content"):
			w.Write([]byte("contents"))
		case r.Method == http.MethodDelete:
			io.WriteString(w, `{"id":"file_1","deleted":true}`)
		case strings.HasSuffix(r.URL.Path, "/files/file_1"):
			io.WriteString(w, `{"id":"file_1","filename":"a"}`)
		}
	})
	defer done()

	ctx := context.Background()
	if files, err := c.Files(ctx); err != nil || len(files) != 1 {
		t.Fatalf("files: %v %+v", err, files)
	}
	if f, err := c.GetFile(ctx, "file_1"); err != nil || f.ID != "file_1" {
		t.Fatalf("get file: %v %+v", err, f)
	}
	if data, err := c.FileContent(ctx, "file_1"); err != nil || string(data) != "contents" {
		t.Fatalf("content: %v %q", err, data)
	}
	if err := c.DeleteFile(ctx, "file_1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestListAndCancelBatch(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/cancel"):
			io.WriteString(w, `{"id":"batch_1","status":"cancelling"}`)
		default:
			io.WriteString(w, `{"data":[{"id":"batch_1","status":"completed"}]}`)
		}
	})
	defer done()

	ctx := context.Background()
	if list, err := c.ListBatches(ctx); err != nil || len(list) != 1 {
		t.Fatalf("list: %v %+v", err, list)
	}
	if b, err := c.CancelBatch(ctx, "batch_1"); err != nil || b.Status != "cancelling" {
		t.Fatalf("cancel: %v %+v", err, b)
	}
}
