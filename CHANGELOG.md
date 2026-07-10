# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.3] - 2026-07-10

### Documentation
- `DOC.md`/`DOC.UK.md` now cover `ResponsesStream`, including how tool calls
  arrive as `output_item.added` / `arguments.delta` / `arguments.done` events.

## [0.1.2] - 2026-07-10

### Added
- `ResponsesStream` now surfaces function-call events: `ResponseStreamEvent`
  gains `Item` (name/call_id), `ItemID`, `OutputIndex` and `Arguments`, so tool
  calls can be read from the responses stream, not just text.

### Changed
- Require `goloop/ai` v0.2.0 (500 no longer retried; jittered backoff).
- `Generate` godoc clarifies it returns the first choice; use `ChatCompletion`
  for n > 1.

## [0.1.1] - 2026-07-09

### Changed
- Require `goloop/ai` v0.1.1, so exhausted retries now surface the provider's
  error body instead of a bare status.

### Added
- Streaming responses API: `ResponsesStream` yields raw `ResponseStreamEvent`
  values (`response.output_text.delta`, `response.completed`, ...) so the newer
  responses endpoint can be consumed token by token, not only synchronously.

### Fixed
- Streamed tool calls are no longer lost when the stream ends without a
  `finish_reason` of `tool_calls` (truncated streams or gateways that omit it).
- Native `ChatCompletion` and `ChatCompletionStream` no longer mutate the
  caller's `ChatRequest`.
- `Generate` no longer drops text when a response returns its content as an
  array of parts (some OpenAI-compatible gateways).
- Error parsing tolerates a numeric `code` field.

## [0.1.0]

Full rewrite on the `github.com/goloop/ai` interface. The old alpha (`v0.0.1-alpha`)
is superseded; the API has been redesigned from scratch for the current OpenAI
API and no longer includes the deprecated edits and legacy fine-tunes endpoints.

### Added
- `Client` implementing `ai.Client`: `Generate` and streaming `Stream` over
  chat completions, with tool use, multimodal image input and system prompts.
- Native `ChatCompletion` and `ChatCompletionStream` exposing the full chat
  option set (response_format, seed, n, ...), and the responses API
  (`CreateResponse`).
- Embeddings (`Embeddings`, `Embed`), image generation (`GenerateImage`), audio
  (`Transcribe`, `Translate`, `Speech`), moderations (`Moderate`), models
  (`Models`, `GetModel`), files (`UploadFile`, `Files`, `GetFile`,
  `FileContent`, `DeleteFile`) and batches (`CreateBatch`, `GetBatch`,
  `ListBatches`, `CancelBatch`).
- Functional options: `WithBaseURL`, `WithHTTPClient`, `WithTimeout`,
  `WithMaxRetries`, `WithHeader`, `WithOrg`, `WithProject`.
- Retries on 429 and 5xx with backoff; normalized `*ai.APIError` errors.
