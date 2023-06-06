package openai

// Check if EmbeddingRequest implements Requester interface.
var _ Requester = (*EmbeddingRequest)(nil)

// EmbeddingRequest represents a request to the OpenAI Embedding API.
type EmbeddingRequest struct {
	// The model ID to use for the request. This is required.
	Model string `json:"model"`

	// The input text to generate embeddings for. This can either be a string
	// or an array of tokens. To generate embeddings for multiple inputs in
	// a single request, pass an array of strings or an array of token arrays.
	// Each input must not exceed 8192 tokens in length. This is required.
	Input interface{} `json:"input"`

	// A unique identifier representing the end-user. This can help OpenAI to
	// monitor and detect abuse. This is optional.
	User string `json:"user,omitempty"`
}

// Embedding represents an individual embedding in the response
// from the OpenAI Embedding API.
type Embedding struct {
	// The object type, which will be "embedding".
	Object string `json:"object"`

	// The actual embedding, represented as an array of floats.
	Embedding []float64 `json:"embedding"`

	// The index of this embedding in the response.
	Index int `json:"index"`
}

// EmbeddingUsage represents usage information in the response from the
// OpenAI Embedding API.
type EmbeddingUsage struct {
	// The number of tokens in the prompt.
	PromptTokens int `json:"prompt_tokens"`

	// The total number of tokens.
	TotalTokens int `json:"total_tokens"`
}

// EmbeddingResponse represents a response from the OpenAI Embedding API.
type EmbeddingResponse struct {
	// The object type, which will be "list".
	Object string `json:"object"`

	// The list of embeddings in the response.
	Data []Embedding `json:"data"`

	// The model ID that was used.
	Model string `json:"model"`

	// Usage information.
	Usage EmbeddingUsage `json:"usage"`
}

// Error returns an error if the request is invalid.
func (r *EmbeddingRequest) Error() error {
	if r.Model == "" {
		return ErrModelRequired
	}

	if r.Input == nil {
		return ErrInputRequired
	}

	return nil
}

// Flush does nothing.
// This is here to satisfy the Requester interface.
func (r *EmbeddingRequest) Flush() {
}
