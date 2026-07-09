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
// It depends only on goloop/ai and the standard library. The chat wire format
// it speaks is the one most other providers copied, so this package doubles as
// the reference for the OpenAI-compatible drivers.
package openai
