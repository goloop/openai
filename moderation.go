package openai

// Check if ModerationRequest implements Requester interface.
var _ Requester = (*ModerationRequest)(nil)

// ModerationRequest represents a request to the OpenAI Moderation API.
type ModerationRequest struct {
	// The input text to classify. This is required.
	Input string `json:"input"`

	// The model to use for the request. Two content moderations models are
	// available: text-moderation-stable and text-moderation-latest.
	// Defaults to text-moderation-latest.
	Model string `json:"model,omitempty"`
}

// ModerationResult represents a single result from the moderation response.
type ModerationResult struct {
	// Map of categories and whether the input was flagged under them.
	Categories map[string]bool `json:"categories"`

	// Map of categories and the associated scores.
	CategoryScores map[string]float64 `json:"category_scores"`

	// Whether the input was flagged under any category.
	Flagged bool `json:"flagged"`
}

// ModerationResponse represents the response from the OpenAI Moderation API.
type ModerationResponse struct {
	// The unique ID of the moderation request.
	ID string `json:"id"`

	// The model that was used for the moderation.
	Model string `json:"model"`

	// The results from the moderation.
	Results []ModerationResult `json:"results"`
}

// Error returns an error if the request is invalid.
func (r *ModerationRequest) Error() error {
	if r.Input == "" {
		return ErrInputRequired
	}

	if r.Model == "" {
		return ErrModelRequired
	}

	return nil
}

// Flush does nothing.
// This is here to satisfy the Requester interface.
func (r *ModerationRequest) Flush() {
}

// IsFlagged returns true if the input was flagged under any category.
func (r *ModerationResponse) IsFlagged() bool {
	for _, result := range r.Results {
		if result.Flagged {
			return true
		}
	}
	return false
}
