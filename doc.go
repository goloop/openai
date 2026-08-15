// Package openai is a client for the OpenAI API, built on the goloop/ai
// interface.
//
// The Client implements ai.Client, so Generate and Stream work the same as
// with any other goloop AI provider. On top of that it exposes OpenAI's native
// endpoints and their full options: chat completions, the responses API
// (synchronous and streaming), embeddings, images, audio (transcription,
// translation and speech), moderations, models, files and batches.
//
//	c := openai.New(os.Getenv("OPENAI_API_KEY"))
//	resp, err := c.Generate(ctx, &ai.Request{
//	    Model:    openai.ModelGPT4oMini,
//	    Messages: []ai.Message{ai.UserText("Say hello in one word.")},
//	})
//
// # Structured output
//
// ai.Request.Format maps onto the provider's response_format, so a request for
// JSON is enforced rather than merely asked for, and ai.Response.JSON decodes
// the reply. Plain JSON mode also puts ai.Format.Instruction into the system
// prompt, because the endpoint rejects it unless the word "json" appears in
// the messages.
//
// # Hosted web search
//
// ai.Request.Hosted maps onto this provider's hosted web search tool:
//
//	resp, err := c.Generate(ctx, &ai.Request{
//	    Model:    openai.ModelGPT4oMini,
//	    Messages: []ai.Message{ai.UserText("What shipped this week?")},
//	    Hosted:   []ai.Hosted{{Kind: ai.HostedWebSearch}},
//	})
//	for _, c := range resp.Citations() { ... }
//
// That tool lives on the responses endpoint, not on chat completions, so
// Generate and Stream go to the responses endpoint when, and only when, a
// request asks for something hosted. Every other call sends the same bytes to
// chat completions as it always did. The two endpoints do not describe an
// outcome in the same words, so ai.Response.StopReason and ai.Usage are
// normalized to the chat vocabulary and a caller never has to know which one
// answered; ai.Response.Raw still holds what the endpoint actually said.
//
// Two things do not survive that endpoint. This provider filters by allowed
// domains only, so ai.HostedWeb.BlockDomains and MaxUses are ai.ErrNoHosted;
// so are ai.Request.Stop sequences, which the responses endpoint has no field
// for and which are too load-bearing to drop quietly.
//
// The sources come back as ai.Citation values on the text they support. This
// provider reports an index range but not what it counts in, so the range is
// kept only where every plausible unit agrees, which is text that is entirely
// ASCII; anywhere else the sources arrive without a range rather than with one
// that might cut a word in half. A stream never carries a range, because the
// indices are counted against an answer that has not finished arriving.
//
// # Images
//
// GenerateImage fits the request to the model: the gpt-image family always
// answers with base64 and rejects response_format, so asking one of those
// models for a URL is refused here rather than by the provider. The gpt-image
// family's own fields (Background, OutputFormat, OutputCompression, Moderation)
// are likewise refused on a dall-e model, before the request is sent, naming
// the field to remove. ImageData.Bytes returns the image itself, and
// ImageResponse.Usage carries the token cost gpt-image reports (nil for dall-e,
// which reports none).
//
// # Asking what this driver can do
//
// Capabilities describes this driver for the decision taken before a call:
// whether to offer a feature at all, and whether it needs one request or two.
//
//	if ai.SupportsHosted(c, ai.Hosted{Kind: ai.HostedWebSearch}) { ... }
//
// It is a hint and not a permission - support also depends on the model, the
// account and the region - so ai.ErrNoHosted and ai.ErrNoFormat remain the
// source of truth and a caller still handles them. What changes is that a
// refusal the provider only reports as a 400 now arrives as those same
// sentinels, wrapped around the original ai.APIError, so one errors.Is covers
// a limitation this driver knew in advance and one it learned over the wire.
//
// It depends only on goloop/ai and the standard library. The chat wire format
// it speaks is the one most other providers copied, so this package doubles as
// the reference for the OpenAI-compatible drivers.
package openai
