package openai

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestTranscribePlainJSONText(t *testing.T) {
	const transcript = `{"text":"spoken JSON"}`
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, transcript)
	})
	defer done()
	got, err := c.Transcribe(context.Background(), &TranscriptionRequest{Model: "m", Format: "text"})
	if err != nil || got != transcript {
		t.Fatalf("got %q, %v", got, err)
	}
}

var responseTextResult string

func BenchmarkResponseText(b *testing.B) {
	for _, n := range []int{1, 16} {
		name := "single"
		if n > 1 {
			name = "multiple"
		}
		b.Run(name, func(b *testing.B) {
			r := ResponsesResponse{Output: []ResponseOutput{{Content: make([]ResponseContent, n)}}}
			for i := range r.Output[0].Content {
				r.Output[0].Content[i] = ResponseContent{Type: "output_text", Text: strings.Repeat("x", 96)}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				responseTextResult = r.Text()
			}
		})
	}
}
