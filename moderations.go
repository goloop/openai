package openai

import "context"

// DefaultModerationModel is used by Moderate when no model is given.
const DefaultModerationModel = "omni-moderation-latest"

// ModerationResult is the classification of one input.
type ModerationResult struct {
	Flagged        bool               `json:"flagged"`
	Categories     map[string]bool    `json:"categories"`
	CategoryScores map[string]float64 `json:"category_scores"`
}

// ModerationResponse is a moderations response.
type ModerationResponse struct {
	ID      string             `json:"id"`
	Model   string             `json:"model"`
	Results []ModerationResult `json:"results"`
}

// Moderate classifies input text and returns its moderation result.
func (c *Client) Moderate(ctx context.Context, input string) (*ModerationResult, error) {
	req := map[string]any{"model": DefaultModerationModel, "input": input}
	var out ModerationResponse
	if err := c.postJSON(ctx, "/moderations", req, &out); err != nil {
		return nil, err
	}
	if len(out.Results) == 0 {
		return &ModerationResult{}, nil
	}
	return &out.Results[0], nil
}
