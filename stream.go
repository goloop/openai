package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"

	"github.com/goloop/ai"
)

// ChatStreamChunk is one streamed chat completions event.
type ChatStreamChunk struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *ChatUsage `json:"usage"`
}

func (c *Client) openStream(ctx context.Context, req *ChatRequest) (*http.Response, error) {
	if req == nil {
		return nil, ai.ErrNoRequest
	}
	r := *req // do not mutate the caller's request
	r.Stream = true
	if r.StreamOptions == nil {
		r.StreamOptions = &streamOptions{IncludeUsage: true}
	}
	body, err := json.Marshal(&r)
	if err != nil {
		return nil, err
	}
	h := c.headers()
	h.Set("accept", "text/event-stream")
	resp, err := c.opts.Do(ctx, http.MethodPost, c.opts.BaseURL+"/chat/completions", body, h)
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

// ChatCompletionStream sends a native streaming chat request and returns an
// iterator over the raw chunks.
func (c *Client) ChatCompletionStream(ctx context.Context, req *ChatRequest) iter.Seq2[ChatStreamChunk, error] {
	return func(yield func(ChatStreamChunk, error) bool) {
		resp, err := c.openStream(ctx, req)
		if err != nil {
			yield(ChatStreamChunk{}, err)
			return
		}
		defer resp.Body.Close()

		for data, err := range ai.SSEEvents(resp.Body) {
			if err != nil {
				yield(ChatStreamChunk{}, err)
				return
			}
			if data == "[DONE]" {
				return
			}
			var chunk ChatStreamChunk
			if e := json.Unmarshal([]byte(data), &chunk); e != nil {
				yield(ChatStreamChunk{}, e)
				return
			}
			if !yield(chunk, nil) {
				return
			}
		}

		// The stream ended without a [DONE] sentinel: it was truncated.
		yield(ChatStreamChunk{}, io.ErrUnexpectedEOF)
	}
}

type toolAcc struct {
	id, name string
	args     []byte
}

// Stream implements [ai.Client] over streaming chat completions.
//
// A request that asks for a hosted capability streams from the responses
// endpoint instead, for the same reason [Client.Generate] does: it is the only
// one that can run one. Splitting only Generate would leave a caller able to
// search in one call and not the other, which is not a shared interface.
func (c *Client) Stream(ctx context.Context, req *ai.Request) iter.Seq2[ai.Chunk, error] {
	if req != nil && len(req.Hosted) > 0 {
		return c.streamHosted(ctx, req)
	}

	return func(yield func(ai.Chunk, error) bool) {
		cr, err := c.chatRequest(req, true)
		if err != nil {
			yield(ai.Chunk{}, err)
			return
		}
		resp, err := c.openStream(ctx, cr)
		if err != nil {
			yield(ai.Chunk{}, wrapUnsupportedCapability(req, err))
			return
		}
		defer resp.Body.Close()

		tools := map[int]*toolAcc{}
		var order []int
		var usage ai.Usage

		flushTools := func() bool {
			for _, idx := range order {
				t := tools[idx]
				input := t.args
				if len(input) == 0 {
					input = []byte("{}")
				}
				if !json.Valid(input) {
					yield(ai.Chunk{}, fmt.Errorf(
						"openai: tool call %q has invalid JSON arguments",
						t.name))
					return false
				}
				call := ai.ToolUse{ID: t.id, Name: t.name, Input: json.RawMessage(input)}
				if !yield(ai.Chunk{ToolCall: &call}, nil) {
					return false
				}
			}
			tools = map[int]*toolAcc{}
			order = nil
			return true
		}

		sawDone := false
		for data, err := range ai.SSEEvents(resp.Body) {
			if err != nil {
				yield(ai.Chunk{}, err)
				return
			}
			if data == "[DONE]" {
				sawDone = true
				break
			}

			var chunk ChatStreamChunk
			if e := json.Unmarshal([]byte(data), &chunk); e != nil {
				yield(ai.Chunk{}, e)
				return
			}
			if chunk.Usage != nil {
				usage = ai.Usage{
					InputTokens:  chunk.Usage.PromptTokens,
					OutputTokens: chunk.Usage.CompletionTokens,
				}
			}

			for _, choice := range chunk.Choices {
				if choice.Delta.Content != "" {
					if !yield(ai.Chunk{Text: choice.Delta.Content, Raw: json.RawMessage(data)}, nil) {
						return
					}
				}
				for _, tc := range choice.Delta.ToolCalls {
					acc := tools[tc.Index]
					if acc == nil {
						acc = &toolAcc{}
						tools[tc.Index] = acc
						order = append(order, tc.Index)
					}
					if tc.ID != "" {
						acc.id = tc.ID
					}
					if tc.Function.Name != "" {
						acc.name = tc.Function.Name
					}
					acc.args = append(acc.args, tc.Function.Arguments...)
				}
				if choice.FinishReason == "tool_calls" {
					if !flushTools() {
						return
					}
				}
			}
		}

		// Flush any tool calls the stream did not close with a
		// finish_reason (truncated streams, gateways that omit it). A false
		// return means the consumer stopped or a tool call had invalid JSON
		// arguments; either way the stream is done.
		if !flushTools() {
			return
		}

		// A stream that ended without a [DONE] sentinel was truncated: the
		// text or tool arguments may be incomplete and the usage partial, so
		// report it rather than presenting a cut-off result as complete.
		if !sawDone {
			yield(ai.Chunk{}, io.ErrUnexpectedEOF)
			return
		}

		final := usage
		yield(ai.Chunk{Done: true, Usage: &final}, nil)
	}
}
