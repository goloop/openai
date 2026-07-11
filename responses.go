package openai

import (
	"context"
	"encoding/json"
	"io"
	"iter"
	"net/http"
	"strings"

	"github.com/goloop/ai"
)

// ResponsesRequest is a request to the responses API, OpenAI's newer stateful
// generation endpoint. Input is a plain string or a structured input array.
type ResponsesRequest struct {
	Model           string   `json:"model"`
	Input           any      `json:"input"`
	Instructions    string   `json:"instructions,omitempty"`
	MaxOutputTokens int      `json:"max_output_tokens,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
	Store           *bool    `json:"store,omitempty"`
	Stream          bool     `json:"stream,omitempty"`
}

// ResponsesResponse is a responses API result.
type ResponsesResponse struct {
	ID     string           `json:"id"`
	Model  string           `json:"model"`
	Status string           `json:"status"`
	Output []ResponseOutput `json:"output"`
	Usage  struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// ResponseOutput is one output item from the responses API.
type ResponseOutput struct {
	Type    string `json:"type"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

// Text returns the concatenation of the output's text segments.
func (r *ResponsesResponse) Text() string {
	var b strings.Builder
	for _, o := range r.Output {
		for _, ct := range o.Content {
			if ct.Type == "output_text" {
				b.WriteString(ct.Text)
			}
		}
	}
	return b.String()
}

// CreateResponse sends a request to the responses API.
func (c *Client) CreateResponse(ctx context.Context, req *ResponsesRequest) (*ResponsesResponse, error) {
	if req == nil {
		return nil, ai.ErrNoRequest
	}
	var out ResponsesResponse
	if err := c.postJSON(ctx, "/responses", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ResponseStreamEvent is one server-sent event of a streaming responses
// request. Type names the event and selects which fields apply:
//
//   - text: "response.output_text.delta" (Delta);
//   - tool call: "response.output_item.added" announces the call (Item, with
//     its name and call_id), "response.function_call_arguments.delta" streams
//     the JSON arguments (Delta, keyed by ItemID), and
//     "response.function_call_arguments.done" carries the full Arguments;
//   - result: "response.completed"/"response.incomplete" (Response);
//   - failure: "response.failed"/"error" (Message, Code).
type ResponseStreamEvent struct {
	Type        string             `json:"type"`
	Delta       string             `json:"delta"`
	Arguments   string             `json:"arguments"`
	ItemID      string             `json:"item_id"`
	OutputIndex int                `json:"output_index"`
	Item        *ResponseItem      `json:"item"`
	Response    *ResponsesResponse `json:"response"`
	Message     string             `json:"message"`
	Code        string             `json:"code"`
}

// ResponseItem is an output item announced by a "response.output_item.added"
// event. For a function call it carries the call's ID, name and the arguments
// accumulated so far.
type ResponseItem struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// openResponsesStream opens the streaming /responses connection for a request.
// The caller owns the returned response body.
func (c *Client) openResponsesStream(ctx context.Context, req *ResponsesRequest) (*http.Response, error) {
	if req == nil {
		return nil, ai.ErrNoRequest
	}
	r := *req // do not mutate the caller's request
	r.Stream = true
	body, err := json.Marshal(&r)
	if err != nil {
		return nil, err
	}
	h := c.headers()
	h.Set("accept", "text/event-stream")
	resp, err := c.opts.Do(ctx, http.MethodPost, c.opts.BaseURL+"/responses", body, h)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		data, _ := readLimited(resp.Body, maxJSONBytes)
		resp.Body.Close()
		return nil, parseError(resp.StatusCode, data)
	}
	return resp, nil
}

// ResponsesStream sends a streaming responses request and yields each raw event
// as it arrives. Text deltas come as "response.output_text.delta" events; the
// final "response.completed" event carries the full result and token usage.
func (c *Client) ResponsesStream(ctx context.Context, req *ResponsesRequest) iter.Seq2[ResponseStreamEvent, error] {
	return func(yield func(ResponseStreamEvent, error) bool) {
		resp, err := c.openResponsesStream(ctx, req)
		if err != nil {
			yield(ResponseStreamEvent{}, err)
			return
		}
		defer resp.Body.Close()

		sawTerminal := false
		for data, err := range ai.SSEEvents(resp.Body) {
			if err != nil {
				yield(ResponseStreamEvent{}, err)
				return
			}
			// The responses API has no [DONE] sentinel; some gateways still
			// emit one, so tolerate it.
			if data == "[DONE]" {
				sawTerminal = true
				return
			}
			var ev ResponseStreamEvent
			if e := json.Unmarshal([]byte(data), &ev); e != nil {
				yield(ResponseStreamEvent{}, e)
				return
			}
			if isTerminalResponseEvent(ev.Type) {
				sawTerminal = true
			}
			if !yield(ev, nil) {
				return
			}
		}

		// A stream that ended without a terminal event (response.completed,
		// response.incomplete, response.failed or error) was truncated.
		if !sawTerminal {
			yield(ResponseStreamEvent{}, io.ErrUnexpectedEOF)
		}
	}
}

// isTerminalResponseEvent reports whether a responses stream event type ends
// the stream, so a stream that stops without one can be flagged as truncated.
func isTerminalResponseEvent(typ string) bool {
	switch typ {
	case "response.completed", "response.incomplete",
		"response.failed", "error":
		return true
	default:
		return false
	}
}
