[![deps.dev](https://img.shields.io/badge/deps.dev-insights-4c8dbc)](https://deps.dev/go/github.com%2Fgoloop%2Fopenai) [![License](https://img.shields.io/badge/license-MIT-brightgreen)](https://github.com/goloop/openai/blob/master/LICENSE) [![License](https://img.shields.io/badge/godoc-YES-green)](https://pkg.go.dev/github.com/goloop/openai) [![Stay with Ukraine](https://img.shields.io/static/v1?label=Stay%20with&message=Ukraine%20♥&color=ffD700&labelColor=0057B8&style=flat)](https://u24.gov.ua/)


# openai

`openai` is a Go client for the OpenAI API. It implements the
`github.com/goloop/ai` interface, so it looks and works like every other goloop
AI provider, and exposes OpenAI's native endpoints with their full options on
top.

## Features

- Chat completions: `Generate` for a single response, `Stream` for
  token-by-token output through `iter.Seq2`.
- Tool use (function calling), multimodal image input and system prompts.
- Native `ChatCompletion` and `ChatCompletionStream` with the full option set
  (response_format, seed, n, ...), plus the responses API.
- Embeddings, image generation, audio (transcription, translation, speech),
  moderations, models, files and batches.
- Retries on 429 and 5xx with backoff; normalized, typed API errors.
- Depends only on `github.com/goloop/ai` and the standard library.

## Installation

```sh
go get github.com/goloop/openai
```

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/goloop/ai"
	"github.com/goloop/openai"
)

func main() {
	c := openai.New(os.Getenv("OPENAI_API_KEY"))

	resp, err := c.Generate(context.Background(), &ai.Request{
		Model:    openai.ModelGPT4oMini,
		Messages: []ai.Message{ai.UserText("Say hello in one word.")},
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(resp.Text())
}
```

## Streaming

```go
for chunk, err := range c.Stream(ctx, req) {
	if err != nil {
		break
	}
	fmt.Print(chunk.Text)
	if chunk.Done && chunk.Usage != nil {
		fmt.Printf("\n[%d in / %d out]\n",
			chunk.Usage.InputTokens, chunk.Usage.OutputTokens)
	}
}
```

## Tools, images and system prompts

Tools, images and system prompts use the same shared `ai` types as every other
provider (see the [reference](DOC.md)). For OpenAI-only options such as
structured output, build a native `ChatRequest`:

```go
resp, _ := c.ChatCompletion(ctx, &openai.ChatRequest{
	Model:          openai.ModelGPT4oMini,
	Messages:       []openai.ChatMessage{{Role: "user", Content: "List two colors as JSON."}},
	ResponseFormat: json.RawMessage(`{"type":"json_object"}`),
})
```

## Native endpoints

```go
c.Embed(ctx, "text-embedding-3-small", "hello", "world")
c.GenerateImage(ctx, &openai.ImageRequest{Model: "gpt-image-1", Prompt: "a cat"})
c.Transcribe(ctx, &openai.TranscriptionRequest{Model: "whisper-1", File: wav, Filename: "a.wav"})
c.Speech(ctx, &openai.SpeechRequest{Model: "gpt-4o-mini-tts", Input: "hi", Voice: "alloy"})
c.Moderate(ctx, "some text")
c.Models(ctx)
c.UploadFile(ctx, "in.jsonl", data, "batch")
c.CreateBatch(ctx, fileID, "/v1/chat/completions", "24h")
c.CreateResponse(ctx, &openai.ResponsesRequest{Model: "gpt-4o-mini", Input: "hi"})
```

The responses API also streams. Range over `ResponsesStream` and read text from
`response.output_text.delta` events; the final `response.completed` event carries
the whole result and token usage:

```go
for ev, err := range c.ResponsesStream(ctx, &openai.ResponsesRequest{
	Model: "gpt-4o-mini", Input: "Tell me a joke.",
}) {
	if err != nil {
		break
	}
	if ev.Type == "response.output_text.delta" {
		fmt.Print(ev.Delta)
	}
}
```

## Documentation

Full reference: **[DOC.md](DOC.md)** (Ukrainian: **[DOC.UK.md](DOC.UK.md)**).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT - see [LICENSE](LICENSE).
