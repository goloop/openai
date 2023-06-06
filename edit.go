package openai

import (
	"strings"
)

// Check if EditRequest implements Requester interface.
var _ Requester = (*EditRequest)(nil)

type EditRequest struct {
	Instruction string  `json:"instruction"`
	Input       string  `json:"input,omitempty"`
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
}

type EditChoice struct {
	Text  string `json:"text"`
	Index int    `json:"index"`
}

type EditUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type EditResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int          `json:"created"`
	Choices []EditChoice `json:"choices"`
	Usage   EditUsage    `json:"usage"`
}

// Error returns an error if the request is invalid.
func (r *EditRequest) Error() error {
	if r.Model == "" {
		return ErrModelRequired
	}

	if r.Instruction == "" {
		return ErrInstructionRequired
	}

	return nil
}

// Flush does nothing.
// This is here to satisfy the Requester interface.
func (r *EditRequest) Flush() {
}

func (r *EditResponse) Text() string {
	var sb strings.Builder

	if len(r.Choices) == 0 {
		return ""
	}

	for i, choice := range r.Choices {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(strings.TrimSpace(choice.Text))
	}

	return sb.String()
}
