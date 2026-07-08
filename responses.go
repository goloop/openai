package openai

import (
	"context"
	"strings"
)

// ResponsesRequest is a request to the responses API, OpenAI's newer stateful
// generation endpoint. Input is a plain string or a structured input array.
type ResponsesRequest struct {
	Model           string   `json:"model"`
	Input           any      `json:"input"`
	Instructions    string   `json:"instructions,omitempty"`
	MaxOutputTokens int      `json:"max_output_tokens,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
	Store           *bool    `json:"store,omitempty"`
}

// ResponsesResponse is a responses API result.
type ResponsesResponse struct {
	ID     string           `json:"id"`
	Model  string           `json:"model"`
	Status string           `json:"status"`
	Output []ResponseOutput `json:"output"`
	Usage  struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// ResponseOutput is one output item from the responses API.
type ResponseOutput struct {
	Type    string `json:"type"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

// Text returns the concatenation of the output's text segments.
func (r *ResponsesResponse) Text() string {
	var b strings.Builder
	for _, o := range r.Output {
		for _, ct := range o.Content {
			if ct.Type == "output_text" {
				b.WriteString(ct.Text)
			}
		}
	}
	return b.String()
}

// CreateResponse sends a request to the responses API.
func (c *Client) CreateResponse(ctx context.Context, req *ResponsesRequest) (*ResponsesResponse, error) {
	var out ResponsesResponse
	if err := c.postJSON(ctx, "/responses", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
