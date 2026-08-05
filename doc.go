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
// # Images
//
// GenerateImage fits the request to the model: the gpt-image family always
// answers with base64 and rejects response_format, so asking one of those
// models for a URL is refused here rather than by the provider. ImageData.Bytes
// returns the image itself.
//
// It depends only on goloop/ai and the standard library. The chat wire format
// it speaks is the one most other providers copied, so this package doubles as
// the reference for the OpenAI-compatible drivers.
package openai
