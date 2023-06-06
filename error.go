package openai

import "errors"

var (
	ErrNoAPIKey     = errors.New("no API key")
	ErrNoAPIBaseURL = errors.New("no API base URL")
	ErrNoHTTPClient = errors.New("no HTTP client")
	ErrNoContext    = errors.New("no context")

	ErrRequestTimedOut = errors.New("request timed out")
	ErrPromptRequired  = errors.New("prompt is required")
	ErrMessageRequired = errors.New("message is required")
	ErrInputRequired   = errors.New("input is required")

	ErrModelRequired = errors.New("model is required")
	ErrImageRequired = errors.New("image is required")

	ErrInvalidResponseFormat = errors.New("invalid response format")
	ErrInvalidSize           = errors.New("invalid size")
	ErrInvalidRole           = errors.New("invalid role")
	ErrInstructionRequired   = errors.New("instruction is required")

	ErrFileRequired    = errors.New("file is required")
	ErrPurposeRequired = errors.New("purpose is required")
)

// Error describes an error data that can be
// returned by the OpenAI API server.
type Error struct {
	Message string `json:"message"` // human-readable text about the error
	Type    string `json:"type"`    // high level error category
	Param   string `json:"param"`   // which parameter the error is related to
	Code    string `json:"code"`    // error code
}

// ErrorResponse is the error response that can be
// returned by the OpenAI API server.
type ErrorResponse struct {
	Error Error `json:"error"` // error details
}
