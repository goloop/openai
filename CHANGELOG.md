# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.1.1] - 2026-08-11

Patch release.

### Documentation
- The reference covers the driver's `Capabilities` and the translation of
  model-level 400 refusals into `ai.ErrNoHosted`/`ai.ErrNoFormat`, which until
  now lived only in the godoc.

## [1.1.0] - 2026-08-11

Minor release, on `ai` v1.1.0.

### Added
- `Capabilities` describes what this driver can be asked for: which hosted
  capabilities it runs, which `ai.HostedWeb` settings it can express, and
  whether a search survives in the same call as a structured format. It is a
  hint for the decision taken before a call, never a substitute for handling
  `ai.ErrNoHosted` - support also depends on the model, the account and the
  region. A test pins the table against what the driver actually refuses, so
  it stays honest mechanically rather than by discipline.
- Web search is reported with the settings the responses endpoint actually
  takes: allowed domains and a region, no use limit and no block list. A search
  and a schema fit in one call there, and `WithFormat` says so.
- A provider refusal that means "this model cannot do that" is now translated
  into the matching sentinel, so an application degrades with one `errors.Is`
  whether the driver knew in advance or learned from a 400. The provider's own
  `ai.APIError` is wrapped, not replaced, and stays reachable with `errors.As`.
  The rules are deliberately narrow - only a 400, only a capability the caller
  asked for, only an error naming that exact feature - because mistaking a
  genuinely bad request for a missing feature would retry it forever.

## [1.0.0] - 2026-08-11

First stable release, on `ai` v1.0.0.

### Added
- `ai.Request.Hosted` maps onto the hosted web search tool. That tool lives on
  the responses endpoint rather than chat completions, so `Generate` and
  `Stream` go to the responses endpoint when, and only when, a request asks for
  something hosted. Every other call sends the same bytes to the same endpoint
  as before.
- The two endpoints do not describe an outcome in the same words, so
  `ai.Response.StopReason` and `ai.Usage` are normalized to the chat
  vocabulary; `ai.Response.Raw` still holds what the endpoint actually said.
- Sources come back as `ai.Citation` values on the text they support. The
  provider reports an index range but not what it counts in, so the range is
  kept only where every plausible unit agrees - text that is entirely ASCII -
  and dropped elsewhere rather than risking a boundary mid-character. A stream
  never carries a range.
- `ai.Response.Hosted` reports whether the search ran and how many times.
- `ResponsesRequest` gained `Tools`, `ToolChoice`, `Text` and `TopP`, without
  which the endpoint could not be asked to search at all.
- `ResponsesResponse` output items, content and annotations are now named types
  carrying the fields a grounded answer uses.

### Changed
- On the hosted path the provider filters by allowed domains only, so
  `ai.HostedWeb.BlockDomains` and `MaxUses` are `ai.ErrNoHosted`; so are
  `ai.Request.Stop` sequences, which the responses endpoint has no field for
  and which are too load-bearing to drop quietly.

## [0.3.0] - 2026-08-05

### Added
- `ai.Request.Format` is mapped onto the provider's `response_format`:
  `ai.FormatJSON` becomes `{"type":"json_object"}` and `ai.FormatJSONSchema`
  becomes `{"type":"json_schema", ...}` with the schema, its name and the
  strict flag. `Response.Format` reports `ai.FormatNative`, since this provider
  enforces every shape it accepts. Until now only the native `ChatRequest`
  could ask for JSON, so callers going through the provider-agnostic interface
  had to strip code fences from the reply by hand.
- Plain JSON mode also appends `ai.Format.Instruction()` to the system prompt.
  The endpoint rejects `json_object` with a 400 unless the word "json" appears
  in the messages; the caller's own system prompt is kept and the instruction
  follows it. Schema mode has no such rule and leaves the prompt untouched.
- `ImageData.Bytes` decodes the image the provider returned inline. It does no
  I/O: an image delivered as a URL returns `ErrNoImageBytes` naming that URL.
- Model constants `ModelGPTImage1`, `ModelDallE3`, `ModelDallE2` and the
  `ImageFormatURL`/`ImageFormatB64JSON` values for `ImageRequest`.

### Changed
- `GenerateImage` fits the request to the model. The `gpt-image` family always
  answers with base64 and rejects `response_format` outright, so the field is
  dropped for those models when it asks for base64; asking one of them for a
  URL now returns `ErrImageFormat` before the request is sent, instead of an
  HTTP 400 from the provider or - worse - a reply whose `URL` is empty. Other
  models are untouched, and the caller's own `ImageRequest` is never modified.
- A nil `ImageRequest` returns `ErrNoImageRequest` instead of being sent as
  `null`.

## [0.2.0] - 2026-07-12

### Added
- `FileContentTo` and `SpeechTo` stream a binary body straight to an
  `io.Writer` instead of buffering it in memory, so a large download or long
  synthesized audio no longer has to fit in a `[]byte`. The existing
  `FileContent` and `Speech` stay as convenience wrappers that read the body
  under a hard ceiling.

### Fixed
- Response bodies are now read under a ceiling (64 MiB for JSON, 128 MiB for
  the in-memory binary wrappers), so a malformed or hostile server cannot
  exhaust memory with an unbounded body.
- A chat stream that ends without a `[DONE]` sentinel, and a responses stream
  that ends without a terminal event, now surface `io.ErrUnexpectedEOF` rather
  than presenting a truncated result as complete.
- The native `ResponsesStream` now surfaces a malformed SSE JSON payload as an
  error instead of silently skipping the event, matching the chat stream.
- A streamed tool call whose accumulated arguments are not valid JSON is now
  reported as an error rather than yielded as an unparseable `Input`.
- `GetModel`, `GetFile`, `FileContent`, `DeleteFile`, `GetBatch` and
  `CancelBatch` now escape the path segment, so an ID with reserved characters
  cannot alter the request URL.
- `ChatCompletion`, `ChatCompletionStream`, `CreateResponse`,
  `ResponsesStream`, `Transcribe`, `Translate` and `Speech` return an error
  instead of panicking when passed a nil request.

### Changed
- Requires `github.com/goloop/ai` v0.3.0.

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
