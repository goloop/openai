package openai

import (
	"encoding/json"

	"github.com/goloop/ai"
)

// parseError turns a non-success response body into an *ai.APIError, filling in
// the provider's error type, code and message when present.
func parseError(status int, body []byte) error {
	var w struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &w)

	return &ai.APIError{
		Status:  status,
		Type:    w.Error.Type,
		Code:    w.Error.Code,
		Message: w.Error.Message,
		Raw:     append(json.RawMessage(nil), body...),
	}
}
