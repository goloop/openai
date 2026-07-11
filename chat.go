package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/goloop/ai"
)

// ChatRequest is the native chat completions request, exposing OpenAI's full
// option set (response_format, seed, n and so on). Build one directly for
// features the shared ai.Request does not model, or let Generate build it.
type ChatRequest struct {
	Model               string          `json:"model"`
	Messages            []ChatMessage   `json:"messages"`
	Tools               []ChatTool      `json:"tools,omitempty"`
	ToolChoice          any             `json:"tool_choice,omitempty"`
	Temperature         *float64        `json:"temperature,omitempty"`
	TopP                *float64        `json:"top_p,omitempty"`
	MaxCompletionTokens int             `json:"max_completion_tokens,omitempty"`
	Stop                []string        `json:"stop,omitempty"`
	N                   int             `json:"n,omitempty"`
	Seed                *int            `json:"seed,omitempty"`
	ResponseFormat      json.RawMessage `json:"response_format,omitempty"`
	User                string          `json:"user,omitempty"`
	Stream              bool            `json:"stream,omitempty"`
	StreamOptions       *streamOptions  `json:"stream_options,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// ChatMessage is one message in a chat request or response. Content is a string
// or a slice of content parts (for images).
type ChatMessage struct {
	Role       string         `json:"role"`
	Content    any            `json:"content,omitempty"`
	Name       string         `json:"name,omitempty"`
	ToolCalls  []ChatToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

// ChatToolCall is a tool call the model produced.
type ChatToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ChatFunctionCall `json:"function"`
}

// ChatFunctionCall is the function name and JSON-encoded arguments of a call.
type ChatFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatTool declares a callable function.
type ChatTool struct {
	Type     string          `json:"type"`
	Function ChatFunctionDef `json:"function"`
}

// ChatFunctionDef is a function's name, description and JSON Schema parameters.
type ChatFunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type contentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

// ChatResponse is the native chat completions response.
type ChatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   ChatUsage    `json:"usage"`
}

// ChatChoice is one completion choice.
type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// ChatUsage reports token usage for a chat completion.
type ChatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatCompletion sends a native chat completions request and returns the whole
// response. Use it for OpenAI-specific options; use Generate for the shared,
// provider-agnostic path.
func (c *Client) ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	out, _, err := c.chatCompletion(ctx, req)
	return out, err
}

func (c *Client) chatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, []byte, error) {
	if req == nil {
		return nil, nil, ai.ErrNoRequest
	}
	r := *req // do not mutate the caller's request
	r.Stream = false
	body, err := json.Marshal(&r)
	if err != nil {
		return nil, nil, err
	}
	data, status, err := c.send(ctx, http.MethodPost, "/chat/completions", body)
	if err != nil {
		return nil, nil, err
	}
	if status != http.StatusOK {
		return nil, data, parseError(status, data)
	}
	var out ChatResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, data, err
	}
	return &out, data, nil
}

// Generate implements [ai.Client] over chat completions. It returns the first
// choice; to request and read several choices (n > 1) use the native
// [Client.ChatCompletion], which exposes every choice.
func (c *Client) Generate(ctx context.Context, req *ai.Request) (*ai.Response, error) {
	cr, err := c.chatRequest(req, false)
	if err != nil {
		return nil, err
	}
	out, raw, err := c.chatCompletion(ctx, cr)
	if err != nil {
		return nil, err
	}
	resp := chatToResponse(out)
	resp.Raw = raw
	return resp, nil
}

// chatRequest converts an ai.Request into a native ChatRequest.
func (c *Client) chatRequest(req *ai.Request, stream bool) (*ChatRequest, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	cr := &ChatRequest{
		Model:               req.Model,
		Messages:            chatMessages(req),
		Temperature:         req.Temperature,
		TopP:                req.TopP,
		MaxCompletionTokens: req.MaxTokens,
		Stop:                req.Stop,
		Stream:              stream,
	}
	for _, t := range req.Tools {
		schema := t.Schema
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object"}`)
		}
		cr.Tools = append(cr.Tools, ChatTool{
			Type: "function",
			Function: ChatFunctionDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  schema,
			},
		})
	}
	if len(cr.Tools) > 0 {
		cr.ToolChoice = chatToolChoice(req.ToolChoice)
	}
	if stream {
		cr.StreamOptions = &streamOptions{IncludeUsage: true}
	}
	return cr, nil
}

func chatMessages(req *ai.Request) []ChatMessage {
	var out []ChatMessage
	if req.System != "" {
		out = append(out, ChatMessage{Role: "system", Content: req.System})
	}
	for _, m := range req.Messages {
		switch m.Role {
		case ai.RoleSystem:
			out = append(out, ChatMessage{Role: "system", Content: partsText(m.Parts)})
		case ai.RoleTool:
			for _, p := range m.Parts {
				if tr, ok := p.(ai.ToolResult); ok {
					out = append(out, ChatMessage{
						Role:       "tool",
						ToolCallID: tr.ID,
						Content:    tr.Content,
					})
				}
			}
		case ai.RoleAssistant:
			msg := ChatMessage{Role: "assistant"}
			var text strings.Builder
			for _, p := range m.Parts {
				switch v := p.(type) {
				case ai.Text:
					text.WriteString(v.Text)
				case ai.ToolUse:
					msg.ToolCalls = append(msg.ToolCalls, ChatToolCall{
						ID:   v.ID,
						Type: "function",
						Function: ChatFunctionCall{
							Name:      v.Name,
							Arguments: string(v.Input),
						},
					})
				}
			}
			if text.Len() > 0 {
				msg.Content = text.String()
			}
			out = append(out, msg)
		default:
			out = append(out, ChatMessage{Role: "user", Content: userContent(m.Parts)})
		}
	}
	return out
}

// userContent returns a plain string when there are no images, or a slice of
// content parts when there are.
func userContent(parts []ai.Part) any {
	hasImage := false
	for _, p := range parts {
		if _, ok := p.(ai.Image); ok {
			hasImage = true
			break
		}
	}
	if !hasImage {
		return partsText(parts)
	}

	var cps []contentPart
	for _, p := range parts {
		switch v := p.(type) {
		case ai.Text:
			cps = append(cps, contentPart{Type: "text", Text: v.Text})
		case ai.Image:
			url := v.URL
			if len(v.Data) > 0 {
				url = "data:" + v.MIME + ";base64," +
					base64.StdEncoding.EncodeToString(v.Data)
			}
			cps = append(cps, contentPart{Type: "image_url", ImageURL: &imageURL{URL: url}})
		}
	}
	return cps
}

func partsText(parts []ai.Part) string {
	var b strings.Builder
	for _, p := range parts {
		if t, ok := p.(ai.Text); ok {
			b.WriteString(t.Text)
		}
	}
	return b.String()
}

// contentText extracts text from a response message's Content, which the API
// returns as a plain string, but OpenAI-compatible gateways may return as an
// array of content parts ([]any of {"type":"text","text":"..."}).
func contentText(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var b strings.Builder
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if t, ok := m["text"].(string); ok {
				b.WriteString(t)
			}
		}
		return b.String()
	default:
		return ""
	}
}

func chatToolChoice(tc ai.ToolChoice) any {
	switch tc {
	case ai.ToolNone:
		return "none"
	case ai.ToolRequired:
		return "required"
	default:
		return "auto"
	}
}

func chatToResponse(cr *ChatResponse) *ai.Response {
	resp := &ai.Response{
		Model: cr.Model,
		Usage: ai.Usage{
			InputTokens:  cr.Usage.PromptTokens,
			OutputTokens: cr.Usage.CompletionTokens,
		},
	}
	if len(cr.Choices) == 0 {
		return resp
	}
	ch := cr.Choices[0]
	resp.StopReason = ch.FinishReason
	if s := contentText(ch.Message.Content); s != "" {
		resp.Parts = append(resp.Parts, ai.Text{Text: s})
	}
	for _, tc := range ch.Message.ToolCalls {
		resp.Parts = append(resp.Parts, ai.ToolUse{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: json.RawMessage(tc.Function.Arguments),
		})
	}
	return resp
}
