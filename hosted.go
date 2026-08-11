package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/goloop/ai"
)

// This file is the responses path of the shared interface.
//
// Chat completions cannot ask this provider to search: the hosted tools live
// on the responses endpoint. So Generate and Stream keep going to
// /chat/completions exactly as before, and switch to /responses only when a
// request asks for something hosted. A caller who asks for nothing hosted
// sends the same bytes to the same endpoint as it always did, which is the
// point: the newer endpoint is worth reaching for what it adds, not worth
// re-routing every existing call through.
//
// The two endpoints do not describe a result in the same words, so what comes
// back is normalized to the vocabulary chat completions already produced.
// ai.Response is the package's contract and it should not depend on which URL
// answered; ai.Response.Raw still holds whatever the endpoint actually said,
// so nothing is hidden, only made comparable.

// webSearchTool is the type of this provider's hosted search tool.
const webSearchTool = "web_search"

// responsesRequest converts an ai.Request into a native ResponsesRequest.
func (c *Client) responsesRequest(req *ai.Request, stream bool) (*ResponsesRequest, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	rr := &ResponsesRequest{
		Model:           req.Model,
		Input:           responsesInput(req),
		Instructions:    systemPrompt(req),
		MaxOutputTokens: req.MaxTokens,
		Temperature:     req.Temperature,
		TopP:            req.TopP,
		Stream:          stream,
	}

	for _, t := range req.Tools {
		schema := t.Schema
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object"}`)
		}
		rr.Tools = append(rr.Tools, ResponseTool{
			Type:        "function",
			Name:        t.Name,
			Description: t.Description,
			Parameters:  schema,
		})
	}
	if len(rr.Tools) > 0 {
		rr.ToolChoice = chatToolChoice(req.ToolChoice)
	}

	hosted, err := hostedTools(req)
	if err != nil {
		return nil, err
	}
	rr.Tools = append(rr.Tools, hosted...)

	// Stop sequences have no field on this endpoint. Dropping them would
	// change where an answer ends without saying so, and a caller who set
	// them meant them.
	if len(req.Stop) > 0 {
		return nil, fmt.Errorf(
			"%w: stop sequences are not available alongside a hosted capability",
			ai.ErrNoHosted)
	}

	format, err := responseFormat(req.Format)
	if err != nil {
		return nil, err
	}
	if len(format) > 0 {
		rr.Text = &ResponseText{Format: responsesFormat(format)}
	}

	return rr, nil
}

// responsesFormat rewrites a chat completions response_format value into the
// shape this endpoint takes. Chat completions nests a schema under
// "json_schema"; here the same fields sit flat inside "format".
func responsesFormat(chat json.RawMessage) json.RawMessage {
	var w struct {
		Type   string `json:"type"`
		Schema *struct {
			Name   string          `json:"name"`
			Schema json.RawMessage `json:"schema"`
			Strict bool            `json:"strict,omitempty"`
		} `json:"json_schema"`
	}
	if err := json.Unmarshal(chat, &w); err != nil || w.Schema == nil {
		return chat
	}

	out, err := json.Marshal(struct {
		Type   string          `json:"type"`
		Name   string          `json:"name"`
		Schema json.RawMessage `json:"schema"`
		Strict bool            `json:"strict,omitempty"`
	}{
		Type:   w.Type,
		Name:   w.Schema.Name,
		Schema: w.Schema.Schema,
		Strict: w.Schema.Strict,
	})
	if err != nil {
		return chat
	}
	return out
}

// hostedTools converts the capabilities a request asked for into this
// provider's own tool entries.
//
// A constraint this endpoint cannot express is [ai.ErrNoHosted] rather than a
// search that runs wider than it was told to: an answer built from sources the
// caller excluded is worse than no answer.
func hostedTools(req *ai.Request) ([]ResponseTool, error) {
	var out []ResponseTool

	for _, h := range req.Hosted {
		if h.Kind != ai.HostedWebSearch {
			return nil, fmt.Errorf("%w: %s", ai.ErrNoHosted, h.Kind)
		}

		t := ResponseTool{Type: webSearchTool}
		if w := h.Web; w != nil {
			switch {
			case w.MaxUses > 0:
				return nil, fmt.Errorf(
					"%w: web search has no use limit", ai.ErrNoHosted)
			case len(w.BlockDomains) > 0:
				return nil, fmt.Errorf(
					"%w: web search filters by allowed domains only",
					ai.ErrNoHosted)
			}
			if len(w.AllowDomains) > 0 {
				t.Filters = &WebSearchFilters{AllowedDomains: w.AllowDomains}
			}
			if w.Region != "" {
				t.UserLocation = &ResponseUserLocation{
					Type:    "approximate",
					Country: w.Region,
				}
			}
		}
		out = append(out, t)
	}

	return out, nil
}

// responsesInput builds the input list for this endpoint. System messages are
// not part of it: they are folded into Instructions, the same way the chat
// path folds them into a system message.
func responsesInput(req *ai.Request) []any {
	var out []any

	for _, m := range req.Messages {
		switch m.Role {
		case ai.RoleSystem:
			continue
		case ai.RoleTool:
			for _, p := range m.Parts {
				if tr, ok := p.(ai.ToolResult); ok {
					out = append(out, map[string]any{
						"type":    "function_call_output",
						"call_id": tr.ID,
						"output":  tr.Content,
					})
				}
			}
		case ai.RoleAssistant:
			out = append(out, assistantInput(m.Parts)...)
		default:
			out = append(out, map[string]any{
				"role":    "user",
				"content": userInput(m.Parts),
			})
		}
	}

	return out
}

// assistantInput renders an assistant turn. A tool call is its own input item
// here rather than a field of the message, which is the shape this endpoint
// reads back.
func assistantInput(parts []ai.Part) []any {
	var out []any
	var content []map[string]any

	for _, p := range parts {
		switch v := p.(type) {
		case ai.Text:
			content = append(content, map[string]any{
				"type": "output_text",
				"text": v.Text,
			})
		case ai.ToolUse:
			args := string(v.Input)
			if args == "" {
				args = "{}"
			}
			out = append(out, map[string]any{
				"type":      "function_call",
				"call_id":   v.ID,
				"name":      v.Name,
				"arguments": args,
			})
		}
	}

	if len(content) > 0 {
		out = append([]any{map[string]any{
			"role":    "assistant",
			"content": content,
		}}, out...)
	}
	return out
}

// userInput renders a user turn's parts as this endpoint's content items.
func userInput(parts []ai.Part) []map[string]any {
	var out []map[string]any
	for _, p := range parts {
		switch v := p.(type) {
		case ai.Text:
			out = append(out, map[string]any{
				"type": "input_text",
				"text": v.Text,
			})
		case ai.Image:
			out = append(out, map[string]any{
				"type":      "input_image",
				"image_url": imageDataURL(v),
			})
		}
	}
	return out
}

// responsesToResponse maps a native responses result onto an ai.Response,
// together with how many times each hosted capability ran.
func responsesToResponse(rr *ResponsesResponse) (*ai.Response, map[ai.HostedKind]int) {
	resp := &ai.Response{
		Model: rr.Model,
		Usage: ai.Usage{
			InputTokens:  rr.Usage.InputTokens,
			OutputTokens: rr.Usage.OutputTokens,
		},
	}

	searches, calls := 0, false
	for _, o := range rr.Output {
		switch o.Type {
		case "message":
			for _, ct := range o.Content {
				if ct.Type != "output_text" {
					continue
				}
				resp.Parts = append(resp.Parts, ai.Text{
					Text:      ct.Text,
					Citations: convAnnotations(ct.Text, ct.Annotations),
				})
			}
		case "function_call":
			calls = true
			resp.Parts = append(resp.Parts, ai.ToolUse{
				ID:    o.CallID,
				Name:  o.Name,
				Input: json.RawMessage(o.Arguments),
			})
		case "web_search_call":
			// Not a tool call: the provider already ran it, and nobody is
			// waiting to answer it. It reaches the caller as the count below
			// and as the citations above.
			searches++
		}
	}

	resp.StopReason = responsesStopReason(rr, calls)

	if searches == 0 {
		return resp, nil
	}
	return resp, map[ai.HostedKind]int{ai.HostedWebSearch: searches}
}

// responsesStopReason renders this endpoint's outcome in the words chat
// completions uses, so that a caller reading ai.Response.StopReason does not
// have to know which endpoint answered.
func responsesStopReason(rr *ResponsesResponse, toolCalls bool) string {
	if rr.Status == "incomplete" {
		reason := ""
		if rr.IncompleteDetails != nil {
			reason = rr.IncompleteDetails.Reason
		}
		switch reason {
		case "max_output_tokens":
			return "length"
		case "content_filter":
			return "content_filter"
		default:
			return "length"
		}
	}
	if toolCalls {
		return "tool_calls"
	}
	if rr.Status == "" {
		return ""
	}
	return "stop"
}

// convAnnotations maps this provider's annotations onto shared citations.
//
// The annotations carry a start and an end index into the text, but not what
// they are counted in, and the answer differs between a provider's own SDKs.
// A range is therefore kept only when text is ASCII, where every plausible
// unit - bytes, runes, UTF-16 code units - is the same number. On anything
// else the sources are reported without a range, which the shared type reads
// as "these back the whole part". A citation that points at the wrong words is
// worse than one that points at all of them, and Ukrainian prose is exactly
// where a wrong guess would land mid-character.
func convAnnotations(text string, as []ResponseAnnotation) []ai.Citation {
	var out []ai.Citation
	ascii := isASCII(text)

	for _, a := range as {
		if a.Type != "url_citation" {
			continue
		}
		c := ai.Citation{URL: a.URL, Title: a.Title}
		if ascii && a.StartIndex >= 0 && a.EndIndex <= len(text) &&
			a.StartIndex <= a.EndIndex {
			c.StartByte = a.StartIndex
			c.EndByte = a.EndIndex
		}
		out = append(out, c)
	}

	return out
}

// isASCII reports whether s is entirely ASCII.
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// imageDataURL renders an image part as the URL this endpoint takes: a remote
// one as it stands, inline bytes as a data URL.
func imageDataURL(v ai.Image) string {
	if len(v.Data) == 0 {
		return v.URL
	}
	return "data:" + v.MIME + ";base64," +
		base64.StdEncoding.EncodeToString(v.Data)
}

// generateHosted answers a request that asked for a hosted capability, over
// the responses endpoint.
func (c *Client) generateHosted(
	ctx context.Context,
	req *ai.Request,
) (*ai.Response, error) {
	rr, err := c.responsesRequest(req, false)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(rr)
	if err != nil {
		return nil, err
	}
	data, status, err := c.send(ctx, http.MethodPost, "/responses", body)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, wrapUnsupportedCapability(req, parseError(status, data))
	}

	var out ResponsesResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}

	resp, calls := responsesToResponse(&out)
	resp.Raw = data
	resp.Format = formatMode(req.Format)
	if resp.Hosted, err = req.HostedReports(calls); err != nil {
		return nil, err
	}
	return resp, nil
}

// streamHosted streams a request that asked for a hosted capability, over the
// responses endpoint. It yields the same kinds of chunk the chat path does,
// plus the sources as their events arrive.
func (c *Client) streamHosted(
	ctx context.Context,
	req *ai.Request,
) iter.Seq2[ai.Chunk, error] {
	return func(yield func(ai.Chunk, error) bool) {
		rr, err := c.responsesRequest(req, true)
		if err != nil {
			yield(ai.Chunk{}, err)
			return
		}

		var usage *ai.Usage
		searches := 0
		args := map[string]*responseCallAcc{}

		for ev, err := range c.ResponsesStream(ctx, rr) {
			if err != nil {
				yield(ai.Chunk{}, wrapUnsupportedCapability(req, err))
				return
			}

			switch ev.Type {
			case "response.output_text.delta":
				if ev.Delta == "" {
					continue
				}
				if !yield(ai.Chunk{Text: ev.Delta}, nil) {
					return
				}

			case "response.output_text.annotation.added":
				// A source arrives in its own event, after the text it
				// supports. It carries no range here: the indices are counted
				// against the finished answer, which does not exist yet.
				if ev.Annotation == nil || ev.Annotation.Type != "url_citation" {
					continue
				}
				if !yield(ai.Chunk{Citations: []ai.Citation{{
					URL:   ev.Annotation.URL,
					Title: ev.Annotation.Title,
				}}}, nil) {
					return
				}

			case "response.output_item.added":
				if ev.Item == nil {
					continue
				}
				switch ev.Item.Type {
				case "function_call":
					args[ev.Item.ID] = &responseCallAcc{
						id:   ev.Item.CallID,
						name: ev.Item.Name,
					}
				case "web_search_call":
					// The provider's own work, not a call to answer.
					searches++
				}

			case "response.function_call_arguments.delta":
				if a := args[ev.ItemID]; a != nil {
					a.buf.WriteString(ev.Delta)
				}

			case "response.function_call_arguments.done":
				a := args[ev.ItemID]
				if a == nil {
					continue
				}
				delete(args, ev.ItemID)
				input := ev.Arguments
				if input == "" {
					input = a.buf.String()
				}
				if input == "" {
					input = "{}"
				}
				if !json.Valid([]byte(input)) {
					yield(ai.Chunk{}, fmt.Errorf(
						"openai: tool call %q has invalid JSON arguments",
						a.name))
					return
				}
				call := ai.ToolUse{
					ID:    a.id,
					Name:  a.name,
					Input: json.RawMessage(input),
				}
				if !yield(ai.Chunk{ToolCall: &call}, nil) {
					return
				}

			case "response.completed", "response.incomplete":
				if ev.Response != nil {
					usage = &ai.Usage{
						InputTokens:  ev.Response.Usage.InputTokens,
						OutputTokens: ev.Response.Usage.OutputTokens,
					}
					// The final event repeats the whole result, which is a
					// more reliable count than the items seen going past.
					if n := countSearches(ev.Response); n > searches {
						searches = n
					}
				}

			case "response.failed", "error":
				yield(ai.Chunk{}, &ai.APIError{
					Type:    ev.Code,
					Message: ev.Message,
				})
				return
			}
		}

		reports, err := req.HostedReports(
			map[ai.HostedKind]int{ai.HostedWebSearch: searches})
		if err != nil {
			yield(ai.Chunk{}, err)
			return
		}
		yield(ai.Chunk{Done: true, Usage: usage, Hosted: reports}, nil)
	}
}

// responseCallAcc accumulates one streamed function call's arguments.
type responseCallAcc struct {
	id, name string
	buf      strings.Builder
}

// countSearches counts the searches a finished result records.
func countSearches(rr *ResponsesResponse) int {
	n := 0
	for _, o := range rr.Output {
		if o.Type == "web_search_call" {
			n++
		}
	}
	return n
}
