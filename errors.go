package openai

import (
	"encoding/json"
	"strings"

	"github.com/goloop/ai"
)

// parseError turns a non-success response body into an *ai.APIError, filling in
// the provider's error type, code and message when present.
func parseError(status int, body []byte) error {
	var w struct {
		Error struct {
			Message string          `json:"message"`
			Type    string          `json:"type"`
			Code    json.RawMessage `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &w)

	return &ai.APIError{
		Status:  status,
		Type:    w.Error.Type,
		Code:    rawToString(w.Error.Code),
		Message: w.Error.Message,
		Raw:     append(json.RawMessage(nil), body...),
	}
}

// rawToString renders a JSON value that may be a string or a number (some
// OpenAI-compatible gateways send a numeric "code") as a plain string.
func rawToString(r json.RawMessage) string {
	s := strings.TrimSpace(string(r))
	if s == "" || s == "null" {
		return ""
	}
	if s[0] == '"' {
		var str string
		if json.Unmarshal(r, &str) == nil {
			return str
		}
	}
	return s
}
