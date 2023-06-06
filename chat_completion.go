package openai

import (
	"strings"

	"github.com/goloop/g"
)

// Check if ChatCompletionRequest implements Requester interface.
var _ Requester = (*ChatCompletionRequest)(nil)

var availableRoleList = []string{"system", "user", "assistant"}

const DefaultRole = "user"

// ChatCompletionRequest is the request to the completions API.
type ChatCompletionRequest struct {
	Messages         []ChatCompletionMessage `json:"messages"`
	Model            string                  `json:"model"`
	MaxTokens        int                     `json:"max_tokens,omitempty"`
	Temperature      float64                 `json:"temperature,omitempty"`
	TopP             float64                 `json:"top_p,omitempty"`
	FrequencyPenalty float64                 `json:"frequency_penalty,omitempty"`
	PresencePenalty  float64                 `json:"presence_penalty,omitempty"`
	LogitBias        map[string]float64      `json:"logit_bias,omitempty"`
}

type ChatCompletionResponse struct {
	ID      string                  `json:"id"`
	Object  string                  `json:"object"`
	Created int64                   `json:"created"`
	Choices []ChatCompletionChoices `json:"choices"`
	Usage   ChatCompletionUsage     `json:"usage"`
}

type ChatCompletionChoices struct {
	Index        int                   `json:"index"`
	Message      ChatCompletionMessage `json:"message"`
	FinishReason string                `json:"finish_reason"`
}

type ChatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

type ChatCompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Error returns an error if the request is invalid.
func (r *ChatCompletionRequest) Error() error {
	if r.Model == "" {
		return ErrModelRequired
	}

	if len(r.Messages) == 0 {
		return ErrMessageRequired
	}

	for _, message := range r.Messages {
		if !g.In(message.Role, availableRoleList...) {
			return ErrInvalidRole
		}

		if message.Content == "" {
			return ErrPromptRequired
		}
	}

	return nil
}

// Flush does nothing.
func (r *ChatCompletionRequest) Flush() {
}

// Text returns the text of the first choice.
func (r *ChatCompletionResponse) Text() string {
	var sb strings.Builder

	if len(r.Choices) == 0 {
		return ""
	}

	for i, choice := range r.Choices {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(strings.TrimSpace(choice.Message.Content))
	}

	return sb.String()
}
