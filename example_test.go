package openai_test

import (
	"encoding/json"
	"fmt"

	"github.com/goloop/ai"
	"github.com/goloop/openai"
)

func ExampleNew() {
	c := openai.New("sk-...")
	_ = c // use c.Generate, c.Stream, c.ChatCompletion, ...
	fmt.Println(openai.ModelGPT4oMini)
	// Output: gpt-4o-mini
}

// ExampleClient_Generate builds a request. Sending it needs a real API key, so
// this example only shows the shape.
func ExampleClient_Generate() {
	req := &ai.Request{
		Model: openai.ModelGPT4oMini,
		Messages: []ai.Message{
			ai.UserText("Name the capital of France."),
		},
	}
	fmt.Println(req.Model, len(req.Messages))
	// Output: gpt-4o-mini 1
}

// ExampleClient_ChatCompletion shows a native request with structured output.
func ExampleClient_ChatCompletion() {
	req := &openai.ChatRequest{
		Model:          openai.ModelGPT4oMini,
		Messages:       []openai.ChatMessage{{Role: "user", Content: "as JSON"}},
		ResponseFormat: json.RawMessage(`{"type":"json_object"}`),
	}
	fmt.Println(req.Model)
	// Output: gpt-4o-mini
}

// ExampleTool shows a tool definition passed with a request.
func ExampleTool() {
	tool := ai.Tool{
		Name:        "get_weather",
		Description: "Get the current weather for a city.",
		Schema:      json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`),
	}
	fmt.Println(tool.Name)
	// Output: get_weather
}
