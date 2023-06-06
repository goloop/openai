package openai

import (
	"strings"
)

// Check if CompletionRequest implements Requester interface.
var _ Requester = (*CompletionRequest)(nil)

// CompletionRequest is the request to the completions API.
type CompletionRequest struct {
	// The prompt(s) to generate completions for, encoded as a string,
	// array of strings, array of tokens, or array of token arrays.
	Prompt interface{} `json:"prompt,omitempty"`
	// Model specifies the ID of the model to use for text generation.
	Model string `json:"model"`

	// Suffix specifies the text to attach at the end of the completion.
	Suffix string `json:"suffix,omitempty"`

	// MaxTokens controls the maximum number of tokens in the generated text.
	MaxTokens int `json:"max_tokens,omitempty"`

	// Temperature affects the randomness of the completion.
	Temperature float64 `json:"temperature,omitempty"`

	// TopP is used for nucleus sampling, influencing the randomness
	// by truncating the token distribution.
	TopP float64 `json:"top_p,omitempty"`

	// N specifies the number of completions to generate.
	N int `json:"n,omitempty"`

	// Stream, when true, returns the results as a stream.
	Stream bool `json:"stream,omitempty"`

	// Logprobs specifies the number of most probable tokens
	// to return with their probabilities.
	Logprobs int `json:"logprobs,omitempty"`

	// Echo, when true, repeats the prompt in the API response.
	Echo bool `json:"echo,omitempty"`

	// Stop defines the sequence where the API will stop
	// generating further tokens.
	Stop interface{} `json:"stop,omitempty"`

	// PresencePenalty penalizes new tokens based on
	// their existing presence in the text.
	PresencePenalty float64 `json:"presence_penalty,omitempty"`

	// FrequencyPenalty penalizes tokens based on their frequency in the text.
	FrequencyPenalty float64 `json:"frequency_penalty,omitempty"`

	// BestOf generates multiple completions and selects the best one.
	BestOf int `json:"best_of,omitempty"`

	// LogitBias allows modifying the likelihood of specified
	// tokens appearing in the completion.
	LogitBias map[string]float64 `json:"logit_bias,omitempty"`

	// User allows to specify a user ID for tracking purposes.
	User string `json:"user,omitempty"`
}

// CompletionChoice is a single completion choice.
type CompletionChoice struct {
	Text         string `json:"text"`
	Index        int    `json:"index"`
	Logprobs     *int   `json:"logprobs"` // can be null
	FinishReason string `json:"finish_reason"`
}

// CompletionUsage is the usage statistics for the completions API.
type CompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// CompletionResponse is the response from the completions API.
type CompletionResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int                `json:"created"`
	Model   string             `json:"model"`
	Choices []CompletionChoice `json:"choices"`
	Usage   CompletionUsage    `json:"usage"`
}

// Error returns an error if the request is invalid.
func (r *CompletionRequest) Error() error {
	if r.Model == "" {
		return ErrModelRequired
	}

	return nil
}

// Flush does nothing.
// This is here to satisfy the Requester interface.
func (r *CompletionRequest) Flush() {
}

// Text returns the generated text.
func (r *CompletionResponse) Text() string {
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
