package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/goloop/ai"
)

// askFor builds a minimal request carrying f.
func askFor(f *ai.Format) *ai.Request {
	return &ai.Request{
		Model:    ModelGPT4oMini,
		System:   "You are terse.",
		Messages: []ai.Message{ai.UserText("Describe this article.")},
		Format:   f,
	}
}

// TestResponseFormatJSON checks plain JSON mode goes out as the provider's own
// json_object, and that the prompt carries the word the endpoint insists on.
func TestResponseFormatJSON(t *testing.T) {
	req := askFor(&ai.Format{Type: ai.FormatJSON})
	cr, err := (&Client{}).chatRequest(req, false)
	if err != nil {
		t.Fatal(err)
	}

	if got := string(cr.ResponseFormat); got != `{"type":"json_object"}` {
		t.Errorf("response_format = %s", got)
	}

	// The endpoint rejects json_object mode unless "json" appears in the
	// messages, so the instruction has to be there - and after the caller's
	// own system prompt, not instead of it.
	system, _ := cr.Messages[0].Content.(string)
	if !strings.HasPrefix(system, "You are terse.") {
		t.Errorf("caller's system prompt was lost: %q", system)
	}
	if !strings.Contains(strings.ToLower(system), "json") {
		t.Errorf("system prompt does not mention json: %q", system)
	}
}

// TestResponseFormatJSONSchema pins the nested shape the provider expects.
func TestResponseFormatJSONSchema(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"a":{"type":"string"}}}`)
	req := askFor(&ai.Format{
		Type:   ai.FormatJSONSchema,
		Name:   "seo",
		Schema: schema,
		Strict: true,
	})

	cr, err := (&Client{}).chatRequest(req, false)
	if err != nil {
		t.Fatal(err)
	}

	var got struct {
		Type   string `json:"type"`
		Schema struct {
			Name   string          `json:"name"`
			Schema json.RawMessage `json:"schema"`
			Strict bool            `json:"strict"`
		} `json:"json_schema"`
	}
	if err := json.Unmarshal(cr.ResponseFormat, &got); err != nil {
		t.Fatalf("response_format is not valid JSON: %v", err)
	}
	if got.Type != "json_schema" {
		t.Errorf("type = %q, want json_schema", got.Type)
	}
	if got.Schema.Name != "seo" {
		t.Errorf("name = %q, want seo", got.Schema.Name)
	}
	if !got.Schema.Strict {
		t.Error("strict was not carried through")
	}
	if string(got.Schema.Schema) != string(schema) {
		t.Errorf("schema = %s, want %s", got.Schema.Schema, schema)
	}

	// Schema mode has no word requirement, so the prompt is left alone.
	if system, _ := cr.Messages[0].Content.(string); system != "You are terse." {
		t.Errorf("system prompt was changed: %q", system)
	}
}

// TestResponseFormatJSONWithoutSystemPrompt covers the shape where the caller
// puts the system prompt in Messages instead of Request.System. The endpoint
// still needs the word "json" somewhere, so the instruction has to arrive as a
// system message of its own rather than be dropped.
func TestResponseFormatJSONWithoutSystemPrompt(t *testing.T) {
	req := &ai.Request{
		Model: ModelGPT4oMini,
		Messages: []ai.Message{
			ai.SystemText("You are terse."),
			ai.UserText("Describe this article."),
		},
		Format: &ai.Format{Type: ai.FormatJSON},
	}

	cr, err := (&Client{}).chatRequest(req, false)
	if err != nil {
		t.Fatal(err)
	}

	var mentions bool
	for _, m := range cr.Messages {
		if s, ok := m.Content.(string); ok && strings.Contains(strings.ToLower(s), "json") {
			mentions = true
		}
	}
	if !mentions {
		t.Errorf("no message mentions json; the endpoint would reject this: %+v",
			cr.Messages)
	}

	// The caller's own system message must still be there, unaltered.
	var kept bool
	for _, m := range cr.Messages {
		if s, ok := m.Content.(string); ok && s == "You are terse." {
			kept = true
		}
	}
	if !kept {
		t.Errorf("the caller's system message was lost: %+v", cr.Messages)
	}
}

// TestStreamSendsTheSameRequest checks streaming asks for the format exactly
// as Generate does. Only the request carries it: a Chunk has no format of its
// own, since how the format was satisfied is a property of the request.
func TestStreamSendsTheSameRequest(t *testing.T) {
	f := &ai.Format{Type: ai.FormatJSON}

	direct, err := (&Client{}).chatRequest(askFor(f), false)
	if err != nil {
		t.Fatal(err)
	}
	streamed, err := (&Client{}).chatRequest(askFor(f), true)
	if err != nil {
		t.Fatal(err)
	}

	if string(direct.ResponseFormat) != string(streamed.ResponseFormat) {
		t.Errorf("stream response_format = %s, direct = %s",
			streamed.ResponseFormat, direct.ResponseFormat)
	}
	if len(direct.Messages) != len(streamed.Messages) {
		t.Errorf("stream sent %d messages, direct sent %d",
			len(streamed.Messages), len(direct.Messages))
	}
}

// TestResponseFormatDefaults checks a request that asks for nothing still
// looks exactly as it did before the field existed.
func TestResponseFormatDefaults(t *testing.T) {
	for _, f := range []*ai.Format{nil, {Type: ai.FormatText}} {
		cr, err := (&Client{}).chatRequest(askFor(f), false)
		if err != nil {
			t.Fatal(err)
		}
		if cr.ResponseFormat != nil {
			t.Errorf("response_format = %s, want it absent", cr.ResponseFormat)
		}
		if system, _ := cr.Messages[0].Content.(string); system != "You are terse." {
			t.Errorf("system prompt was changed: %q", system)
		}
	}
}

// TestResponseFormatRejectsUnknown checks a format this driver cannot render
// fails here, not as a puzzling error from the provider.
func TestResponseFormatRejectsUnknown(t *testing.T) {
	req := askFor(&ai.Format{Type: ai.FormatType(99)})
	if _, err := (&Client{}).chatRequest(req, false); !errors.Is(err, ai.ErrBadFormat) {
		t.Errorf("err = %v, want %v", err, ai.ErrBadFormat)
	}

	req = askFor(&ai.Format{Type: ai.FormatJSONSchema})
	if _, err := (&Client{}).chatRequest(req, false); !errors.Is(err, ai.ErrNoSchema) {
		t.Errorf("err = %v, want %v", err, ai.ErrNoSchema)
	}
}

// TestGenerateReportsFormatMode checks the caller can tell an enforced answer
// from a requested one. This provider enforces every shape it accepts.
func TestGenerateReportsFormatMode(t *testing.T) {
	body := `{"id":"1","model":"gpt-4o-mini","choices":[{"index":0,` +
		`"message":{"role":"assistant","content":"{\"a\":\"x\"}"},` +
		`"finish_reason":"stop"}]}`

	cases := map[string]struct {
		format *ai.Format
		want   ai.FormatMode
	}{
		"nothing asked": {nil, ai.FormatNone},
		"text":          {&ai.Format{Type: ai.FormatText}, ai.FormatNone},
		"json":          {&ai.Format{Type: ai.FormatJSON}, ai.FormatNative},
		"json schema": {&ai.Format{
			Type:   ai.FormatJSONSchema,
			Schema: json.RawMessage(`{"type":"object"}`),
		}, ai.FormatNative},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			client, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				io.Copy(io.Discard, r.Body)
				w.Write([]byte(body))
			})
			defer done()

			resp, err := client.Generate(context.Background(), askFor(c.format))
			if err != nil {
				t.Fatal(err)
			}
			if resp.Format != c.want {
				t.Errorf("Format = %s, want %s", resp.Format, c.want)
			}
		})
	}
}

// TestGenerateJSONRoundTrip is the whole point at the call site: ask for JSON,
// decode it, with no fence-stripping in the application.
func TestGenerateJSONRoundTrip(t *testing.T) {
	client, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.Write([]byte(`{"id":"1","model":"gpt-4o-mini","choices":[{"index":0,` +
			`"message":{"role":"assistant","content":"{\"title\":\"Autumn\"}"},` +
			`"finish_reason":"stop"}]}`))
	})
	defer done()

	resp, err := client.Generate(context.Background(),
		askFor(&ai.Format{Type: ai.FormatJSON}))
	if err != nil {
		t.Fatal(err)
	}

	var seo struct {
		Title string `json:"title"`
	}
	if err := resp.JSON(&seo); err != nil {
		t.Fatalf("JSON() = %v", err)
	}
	if seo.Title != "Autumn" {
		t.Errorf("title = %q, want Autumn", seo.Title)
	}
}

// TestImagePayloadFitsTheModel covers the incompatibility the application used
// to carry as a comment: gpt-image rejects response_format outright.
func TestImagePayloadFitsTheModel(t *testing.T) {
	cases := []struct {
		name  string
		req   *ImageRequest
		want  string // the response_format that goes on the wire
		fails bool
	}{
		{
			name: "gpt-image drops an explicit base64 request",
			req:  &ImageRequest{Model: ModelGPTImage1, ResponseFormat: ImageFormatB64JSON},
			want: "",
		},
		{
			name: "gpt-image sends nothing when nothing was asked",
			req:  &ImageRequest{Model: ModelGPTImage1},
			want: "",
		},
		{
			name:  "gpt-image cannot return a URL",
			req:   &ImageRequest{Model: ModelGPTImage1, ResponseFormat: ImageFormatURL},
			fails: true,
		},
		{
			name:  "an unknown format is not guessed at either",
			req:   &ImageRequest{Model: ModelGPTImage1, ResponseFormat: "webp"},
			fails: true,
		},
		{
			name: "a future gpt-image snapshot is covered",
			req:  &ImageRequest{Model: "gpt-image-2-mini", ResponseFormat: ImageFormatB64JSON},
			want: "",
		},
		{
			name:  "a bare family name counts",
			req:   &ImageRequest{Model: "gpt-image", ResponseFormat: ImageFormatURL},
			fails: true,
		},
		{
			name: "a model that merely starts the same way does not",
			req:  &ImageRequest{Model: "gpt-imagerouter", ResponseFormat: ImageFormatURL},
			want: ImageFormatURL,
		},
		{
			name: "dall-e keeps its url",
			req:  &ImageRequest{Model: ModelDallE3, ResponseFormat: ImageFormatURL},
			want: ImageFormatURL,
		},
		{
			name: "dall-e keeps its base64",
			req:  &ImageRequest{Model: ModelDallE3, ResponseFormat: ImageFormatB64JSON},
			want: ImageFormatB64JSON,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := c.req.ResponseFormat
			got, err := imagePayload(c.req)

			if c.fails {
				if !errors.Is(err, ErrImageFormat) {
					t.Fatalf("err = %v, want %v", err, ErrImageFormat)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.ResponseFormat != c.want {
				t.Errorf("response_format = %q, want %q", got.ResponseFormat, c.want)
			}
			if c.req.ResponseFormat != before {
				t.Errorf("the caller's own request was modified: %q -> %q",
					before, c.req.ResponseFormat)
			}
		})
	}

	if _, err := imagePayload(nil); !errors.Is(err, ErrNoImageRequest) {
		t.Errorf("nil request: err = %v, want %v", err, ErrNoImageRequest)
	}
}

// TestGenerateImageOmitsFormat checks the adjustment reaches the wire.
func TestGenerateImageOmitsFormat(t *testing.T) {
	var sent map[string]any
	client, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &sent)
		w.Write([]byte(`{"created":1,"data":[{"b64_json":"aGk="}]}`))
	})
	defer done()

	resp, err := client.GenerateImage(context.Background(), &ImageRequest{
		Model:          ModelGPTImage1,
		Prompt:         "a cat",
		ResponseFormat: ImageFormatB64JSON,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := sent["response_format"]; ok {
		t.Errorf("response_format was sent to a model that rejects it: %v", sent)
	}

	got, err := resp.Data[0].Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hi" {
		t.Errorf("bytes = %q, want %q", got, "hi")
	}
}

// TestImageDataBytes covers reading the image, including the case the accessor
// deliberately refuses to fetch.
func TestImageDataBytes(t *testing.T) {
	want := []byte{0x89, 'P', 'N', 'G'}
	inline := ImageData{B64JSON: base64.StdEncoding.EncodeToString(want)}
	got, err := inline.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("bytes = %v, want %v", got, want)
	}

	linked := ImageData{URL: "https://example.com/a.png"}
	if _, err := linked.Bytes(); !errors.Is(err, ErrNoImageBytes) {
		t.Errorf("err = %v, want %v", err, ErrNoImageBytes)
	} else if !strings.Contains(err.Error(), "https://example.com/a.png") {
		t.Errorf("error does not name the URL to fetch: %v", err)
	}

	if _, err := (ImageData{}).Bytes(); !errors.Is(err, ErrNoImageBytes) {
		t.Errorf("empty image: err = %v, want %v", err, ErrNoImageBytes)
	}

	if _, err := (ImageData{B64JSON: "not base64!"}).Bytes(); err == nil {
		t.Error("broken base64 decoded without error")
	}
}
