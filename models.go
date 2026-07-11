package openai

import (
	"context"
	"net/url"
)

// Model describes a model returned by the models endpoint.
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// Models lists the models available to the account.
func (c *Client) Models(ctx context.Context) ([]Model, error) {
	var out struct {
		Data []Model `json:"data"`
	}
	if err := c.getJSON(ctx, "/models", &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// GetModel returns a single model by ID.
func (c *Client) GetModel(ctx context.Context, id string) (*Model, error) {
	var m Model
	if err := c.getJSON(ctx, "/models/"+url.PathEscape(id), &m); err != nil {
		return nil, err
	}
	return &m, nil
}
