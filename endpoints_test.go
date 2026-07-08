package openai

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestEmbed(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/embeddings") {
			t.Errorf("path = %q", r.URL.Path)
		}
		io.WriteString(w, `{"model":"m","data":[{"index":0,"embedding":[0.1,0.2]},`+
			`{"index":1,"embedding":[0.3,0.4]}],"usage":{"prompt_tokens":2,"total_tokens":2}}`)
	})
	defer done()

	vecs, err := c.Embed(context.Background(), "m", "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 2 || vecs[0][1] != 0.2 || vecs[1][0] != 0.3 {
		t.Fatalf("vecs = %+v", vecs)
	}
}

func TestModels(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"data":[{"id":"gpt-4o","object":"model","owned_by":"openai"}]}`)
	})
	defer done()

	models, err := c.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].ID != "gpt-4o" {
		t.Fatalf("models = %+v", models)
	}
}

func TestModerate(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"results":[{"flagged":true,"categories":{"hate":false},`+
			`"category_scores":{"hate":0.01}}]}`)
	})
	defer done()

	res, err := c.Moderate(context.Background(), "text")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Flagged {
		t.Errorf("flagged = %v", res.Flagged)
	}
}

func TestGenerateImage(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/images/generations") {
			t.Errorf("path = %q", r.URL.Path)
		}
		io.WriteString(w, `{"created":1,"data":[{"url":"https://img/a.png"}]}`)
	})
	defer done()

	resp, err := c.GenerateImage(context.Background(), &ImageRequest{
		Model: "gpt-image-1", Prompt: "a cat", N: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) != 1 || resp.Data[0].URL != "https://img/a.png" {
		t.Fatalf("data = %+v", resp.Data)
	}
}

func TestSpeech(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte{0x49, 0x44, 0x33}) // "ID3" audio header bytes
	})
	defer done()

	audio, err := c.Speech(context.Background(), &SpeechRequest{
		Model: "gpt-4o-mini-tts", Input: "hello", Voice: "alloy",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(audio) != 3 {
		t.Errorf("audio len = %d", len(audio))
	}
}

func TestTranscribe(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Errorf("content-type = %q", r.Header.Get("Content-Type"))
		}
		io.WriteString(w, `{"text":"transcribed"}`)
	})
	defer done()

	text, err := c.Transcribe(context.Background(), &TranscriptionRequest{
		Model: "whisper-1", File: []byte("audio"), Filename: "a.mp3",
	})
	if err != nil {
		t.Fatal(err)
	}
	if text != "transcribed" {
		t.Errorf("text = %q", text)
	}
}

func TestFilesAndBatches(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/files") && r.Method == http.MethodPost:
			io.WriteString(w, `{"id":"file_1","object":"file","purpose":"batch"}`)
		case strings.HasSuffix(r.URL.Path, "/batches") && r.Method == http.MethodPost:
			io.WriteString(w, `{"id":"batch_1","status":"validating",`+
				`"input_file_id":"file_1","request_counts":{"total":1}}`)
		case strings.HasSuffix(r.URL.Path, "/batches/batch_1"):
			io.WriteString(w, `{"id":"batch_1","status":"completed",`+
				`"output_file_id":"file_out","request_counts":{"total":1,"completed":1}}`)
		}
	})
	defer done()

	ctx := context.Background()
	f, err := c.UploadFile(ctx, "in.jsonl", []byte(`{}`), "batch")
	if err != nil || f.ID != "file_1" {
		t.Fatalf("upload: %v %+v", err, f)
	}
	b, err := c.CreateBatch(ctx, f.ID, "/v1/chat/completions", "24h")
	if err != nil || b.ID != "batch_1" {
		t.Fatalf("create batch: %v %+v", err, b)
	}
	got, err := c.GetBatch(ctx, "batch_1")
	if err != nil || got.Status != "completed" || got.RequestCounts.Completed != 1 {
		t.Fatalf("get batch: %v %+v", err, got)
	}
}
