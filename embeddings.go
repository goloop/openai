package openai

import "context"

// EmbeddingRequest is an embeddings request.
type EmbeddingRequest struct {
	Model          string   `json:"model"`
	Input          []string `json:"input"`
	Dimensions     int      `json:"dimensions,omitempty"`
	EncodingFormat string   `json:"encoding_format,omitempty"`
	User           string   `json:"user,omitempty"`
}

// EmbeddingResponse is an embeddings response.
type EmbeddingResponse struct {
	Object string      `json:"object"`
	Model  string      `json:"model"`
	Data   []Embedding `json:"data"`
	Usage  struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// Embedding is one embedding vector.
type Embedding struct {
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

// Embeddings returns embedding vectors for the request's inputs.
func (c *Client) Embeddings(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error) {
	var out EmbeddingResponse
	if err := c.postJSON(ctx, "/embeddings", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Embed is a convenience that returns the embedding vectors for the given
// inputs, in order.
func (c *Client) Embed(ctx context.Context, model string, input ...string) ([][]float64, error) {
	resp, err := c.Embeddings(ctx, &EmbeddingRequest{Model: model, Input: input})
	if err != nil {
		return nil, err
	}
	out := make([][]float64, len(resp.Data))
	for _, e := range resp.Data {
		if e.Index >= 0 && e.Index < len(out) {
			out[e.Index] = e.Embedding
		}
	}
	return out, nil
}
