package openai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/goloop/ai"
)

// gpt-image returns a usage block; it must be decoded, because image generation
// is billed apart from text and this is the only place its cost shows up.
func TestImageResponseCarriesUsage(t *testing.T) {
	client, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"created":1,"data":[{"b64_json":"aGk="}],`+
			`"usage":{"total_tokens":120,"input_tokens":20,"output_tokens":100,`+
			`"input_tokens_details":{"text_tokens":8,"image_tokens":12}}}`)
	})
	defer done()

	resp, err := client.GenerateImage(context.Background(), &ImageRequest{
		Model: ModelGPTImage1, Prompt: "a cat",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Usage == nil {
		t.Fatal("usage was dropped")
	}
	if resp.Usage.TotalTokens != 120 || resp.Usage.OutputTokens != 100 {
		t.Errorf("usage = %+v", resp.Usage)
	}
	if d := resp.Usage.InputTokensDetails; d == nil || d.ImageTokens != 12 {
		t.Errorf("input token details = %+v", d)
	}
}

// A response with no usage - dall-e never returns one - leaves Usage nil, so a
// caller can tell "not reported" from a zero count.
func TestImageResponseWithoutUsage(t *testing.T) {
	client, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"created":1,"data":[{"url":"https://img/1"}]}`)
	})
	defer done()

	resp, err := client.GenerateImage(context.Background(), &ImageRequest{
		Model: ModelDallE3, Prompt: "a cat",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Usage != nil {
		t.Errorf("usage = %+v, want nil for a provider that does not report it",
			resp.Usage)
	}
}

// The gpt-image-only fields reach the wire on a gpt-image model.
func TestGPTImageParamsReachTheWire(t *testing.T) {
	var sent map[string]any
	client, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &sent)
		io.WriteString(w, `{"created":1,"data":[{"b64_json":"aGk="}]}`)
	})
	defer done()

	zero := 0
	_, err := client.GenerateImage(context.Background(), &ImageRequest{
		Model:             ModelGPTImage1,
		Prompt:            "a cat",
		Background:        "transparent",
		OutputFormat:      "webp",
		OutputCompression: &zero,
		Moderation:        "low",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"background", "output_format", "moderation"} {
		if _, ok := sent[k]; !ok {
			t.Errorf("%s was not sent: %v", k, sent)
		}
	}
	// output_compression = 0 must be sent, not omitted: 0 is a real value,
	// which is why the field is a pointer.
	if v, ok := sent["output_compression"]; !ok || v.(float64) != 0 {
		t.Errorf("output_compression = %v (present=%v), want 0 sent", v, ok)
	}
}

// The gpt-image-only fields are refused on a dall-e model, before the network,
// and the error names the field to remove.
func TestGPTImageParamsRejectedForDallE(t *testing.T) {
	fields := []struct {
		name string
		set  func(*ImageRequest)
	}{
		{"background", func(r *ImageRequest) { r.Background = "opaque" }},
		{"output_format", func(r *ImageRequest) { r.OutputFormat = "webp" }},
		{"output_compression", func(r *ImageRequest) { z := 50; r.OutputCompression = &z }},
		{"moderation", func(r *ImageRequest) { r.Moderation = "low" }},
	}

	for _, f := range fields {
		t.Run(f.name, func(t *testing.T) {
			req := &ImageRequest{Model: ModelDallE3, Prompt: "a cat"}
			f.set(req)
			_, err := imagePayload(req)
			if !errors.Is(err, ErrImageFormat) {
				t.Fatalf("err = %v, want ErrImageFormat", err)
			}
		})
	}
}

// The driver reports image support, so a UI can offer the control.
func TestOpenAIReportsImageSupport(t *testing.T) {
	if !ai.SupportsImages(New("k")) {
		t.Error("openai should report image support")
	}
}

// A request that sets none of the new fields is untouched by the fitting -
// existing calls keep working exactly as before.
func TestImagePayloadLeavesOrdinaryRequestsAlone(t *testing.T) {
	req := &ImageRequest{Model: ModelDallE3, Prompt: "a cat", Size: "1024x1024"}
	got, err := imagePayload(req)
	if err != nil {
		t.Fatal(err)
	}
	if got != req {
		t.Error("an ordinary request was copied or altered")
	}
}
