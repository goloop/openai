# openai - reference

The full reference for the `openai` package: the client, the shared `goloop/ai`
model, chat completions (interface and native), streaming, the responses API,
embeddings, images, audio, moderations, models, files and batches.

Ukrainian version: **[DOC.UK.md](DOC.UK.md)**.

## Contents

- [Mental model](#mental-model)
- [Creating a client](#creating-a-client)
- [Generate and Stream](#generate-and-stream)
- [Structured output](#structured-output)
- [Native chat completions](#native-chat-completions)
- [Responses API](#responses-api)
- [Embeddings](#embeddings)
- [Images](#images)
- [Audio](#audio)
- [Moderations](#moderations)
- [Models](#models)
- [Files](#files)
- [Batches](#batches)
- [Options and errors](#options-and-errors)

## Mental model

`openai.Client` implements `ai.Client`, the provider-agnostic contract from
`github.com/goloop/ai`. The shared `Generate` and `Stream` cover the common
ground - chat with tools, images and streaming - so code written against the
interface runs on any provider.

OpenAI-specific power lives in native methods: the full `ChatCompletion`
request, the responses API, embeddings, images, audio, moderations, files and
batches. Those are not part of the shared interface.

```go
import (
	"github.com/goloop/ai"
	"github.com/goloop/openai"
)
```

## Creating a client

```go
c := openai.New(os.Getenv("OPENAI_API_KEY"))

c = openai.New(apiKey,
	openai.WithOrg("org-..."),
	openai.WithProject("proj-..."),
	openai.WithTimeout(30*time.Second),
)
```

The base URL defaults to `https://api.openai.com/v1`. Point `WithBaseURL` at any
OpenAI-compatible endpoint to reuse this client against another gateway.

## Generate and Stream

```go
resp, err := c.Generate(ctx, &ai.Request{
	Model:    openai.ModelGPT4oMini,
	System:   "You are concise.",
	Messages: []ai.Message{ai.UserText("Name three primary colors.")},
})
resp.Text()
resp.ToolCalls()
resp.Usage
```

`Stream` returns `iter.Seq2[ai.Chunk, error]`: text deltas as chunks with
`Text`, a finished tool call as a chunk with `ToolCall`, and a final chunk with
`Done` and `Usage`.

```go
for chunk, err := range c.Stream(ctx, req) {
	if err != nil {
		return err
	}
	fmt.Print(chunk.Text)
}
```

Tool use, images and system prompts use the shared `ai` types: `ai.Tool`,
`ai.Image`, `ai.ToolResult` and a `RoleSystem` message or the `System` field.
Tool results are sent back as `RoleTool` messages whose `ai.ToolResult.ID`
matches the `ai.ToolUse.ID`.

## Structured output

`ai.Request.Format` maps onto the provider's own `response_format`, so a
request for JSON is enforced by the provider rather than merely asked for:

```go
resp, err := c.Generate(ctx, &ai.Request{
	Model:    openai.ModelGPT4oMini,
	Messages: []ai.Message{ai.UserText("Draft SEO fields for this article.")},
	Format: &ai.Format{
		Type:   ai.FormatJSONSchema,
		Name:   "seo",
		Schema: schema,
		Strict: true,
	},
})

var seo SEO
err = resp.JSON(&seo)
```

| `ai.Format.Type` | sent as |
|---|---|
| `ai.FormatJSON` | `{"type":"json_object"}` |
| `ai.FormatJSONSchema` | `{"type":"json_schema","json_schema":{name, schema, strict}}` |

`Response.Format` is `ai.FormatNative` for both: this provider enforces every
shape it accepts.

Two things worth knowing:

- **Plain JSON mode needs the word in the prompt.** The endpoint rejects
  `json_object` with a 400 unless "json" appears somewhere in the messages, so
  for `ai.FormatJSON` the driver appends `ai.Format.Instruction()` to the system
  prompt. Your own system prompt is kept, and the instruction follows it. Schema
  mode carries no such rule and the prompt is left untouched.
- **Schema mode needs a recent model.** There is no model capability table here
  on purpose - one would be wrong the week a model ships - so an unsupported
  pairing is reported by the provider, which says so plainly.

## Native chat completions

For OpenAI-only options build a `ChatRequest` and call `ChatCompletion` or
`ChatCompletionStream`:

```go
resp, err := c.ChatCompletion(ctx, &openai.ChatRequest{
	Model:          openai.ModelGPT4oMini,
	Messages:       []openai.ChatMessage{{Role: "user", Content: "as JSON"}},
	ResponseFormat: json.RawMessage(`{"type":"json_object"}`),
	Seed:           ptr(42),
})
```

`ChatMessage.Content` is a string or a slice of content parts; `Tools`,
`ToolChoice`, `Temperature`, `TopP`, `MaxCompletionTokens`, `Stop`, `N`, `Seed`,
`ResponseFormat` and `User` are all available.

## Responses API

```go
resp, err := c.CreateResponse(ctx, &openai.ResponsesRequest{
	Model:        openai.ModelGPT4oMini,
	Input:        "Write a haiku about Go.",
	Instructions: "Be poetic.",
})
resp.Text()
```

`ResponsesStream` streams the same request as raw `ResponseStreamEvent` values.
The `Type` field names each event and selects which fields apply:

```go
for ev, err := range c.ResponsesStream(ctx, req) {
	if err != nil {
		break
	}
	switch ev.Type {
	case "response.output_text.delta":
		fmt.Print(ev.Delta) // incremental text
	case "response.output_item.added":
		// ev.Item announces a tool call (Name, CallID)
	case "response.function_call_arguments.delta":
		// ev.Delta streams the JSON arguments, keyed by ev.ItemID
	case "response.function_call_arguments.done":
		// ev.Arguments holds the full arguments object
	case "response.completed":
		// ev.Response carries the final result and usage
	}
}
```

Tool calls arrive as a sequence of `response.output_item.added` (the
`ResponseItem` with its name and `call_id`), `...arguments.delta` (streamed JSON)
and `...arguments.done` (the complete `Arguments`). A `response.failed` or
`error` event carries `Message` and `Code`.

## Embeddings

```go
vecs, err := c.Embed(ctx, "text-embedding-3-small", "hello", "world")
// or the full request:
resp, err := c.Embeddings(ctx, &openai.EmbeddingRequest{
	Model: "text-embedding-3-small", Input: []string{"hello"}, Dimensions: 256,
})
```

## Images

```go
resp, err := c.GenerateImage(ctx, &openai.ImageRequest{
	Model: openai.ModelGPTImage1, Prompt: "a watercolor cat", Size: "1024x1024",
})
png, err := resp.Data[0].Bytes() // the raw image; URL and B64JSON stay available
```

`Bytes` decodes what the provider sent inline. It does no I/O: an image that
came back as a URL returns `ErrNoImageBytes` naming the URL, because fetching
it is a network call with your timeouts and proxy rules, not a decision for a
field accessor.

**The request is fitted to the model.** The `gpt-image` family always answers
with base64 and rejects `response_format` outright, while `dall-e-2`/`dall-e-3`
accept it and default to a URL:

| Model | `ResponseFormat` | Result |
|---|---|---|
| `gpt-image-*` | unset or `ImageFormatB64JSON` | field dropped; base64 comes back |
| `gpt-image-*` | `ImageFormatURL` | `ErrImageFormat`, before anything is sent |
| `dall-e-*` | anything | passed through unchanged |

Asking a `gpt-image` model for a URL fails here rather than succeeding with an
empty `URL` field, so the incompatibility is not something to rediscover by
trial and error. Your own `ImageRequest` value is never modified.

## Audio

```go
text, err := c.Transcribe(ctx, &openai.TranscriptionRequest{
	Model: "whisper-1", File: wav, Filename: "speech.wav",
})
text, err = c.Translate(ctx, &openai.TranscriptionRequest{
	Model: "whisper-1", File: wav, Filename: "speech.wav",
})
audio, err := c.Speech(ctx, &openai.SpeechRequest{
	Model: "gpt-4o-mini-tts", Input: "Hello", Voice: "alloy",
})
```

`Speech` returns the raw audio bytes, read under a hard ceiling; for a long
input use `SpeechTo(ctx, req, w)` to stream the audio to an `io.Writer` without
buffering it in memory. Likewise `FileContentTo(ctx, id, w)` streams a file
download, while `FileContent` returns it as a capped `[]byte`.

## Moderations

```go
res, err := c.Moderate(ctx, "some text")
res.Flagged
res.Categories     // map[string]bool
res.CategoryScores // map[string]float64
```

## Models

```go
models, err := c.Models(ctx)
m, err := c.GetModel(ctx, openai.ModelGPT4o)
```

## Files

```go
f, err := c.UploadFile(ctx, "input.jsonl", data, "batch")
files, err := c.Files(ctx)
f, err = c.GetFile(ctx, f.ID)
data, err := c.FileContent(ctx, f.ID)
err = c.DeleteFile(ctx, f.ID)
```

## Batches

Upload a JSONL file of requests, then run it against an endpoint:

```go
in, _ := c.UploadFile(ctx, "batch.jsonl", jsonl, "batch")
b, err := c.CreateBatch(ctx, in.ID, "/v1/chat/completions", "24h")
b, err = c.GetBatch(ctx, b.ID)            // poll b.Status
// results are in the file b.OutputFileID:
out, err := c.FileContent(ctx, b.OutputFileID)

batches, err := c.ListBatches(ctx)
b, err = c.CancelBatch(ctx, b.ID)
```

## Options and errors

Shared options: `WithBaseURL`, `WithHTTPClient`, `WithTimeout`,
`WithMaxRetries`, `WithHeader`. OpenAI-specific: `WithOrg`, `WithProject`.

A non-success response becomes an `*ai.APIError` with `Status`, `Type`, `Code`,
`Message` and the raw body:

```go
var apiErr *ai.APIError
if errors.As(err, &apiErr) && apiErr.Status == http.StatusTooManyRequests {
	// back off
}
```

Requests missing a model or messages fail before the network with
`ai.ErrNoModel` or `ai.ErrNoMessages`.
