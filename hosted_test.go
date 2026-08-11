package openai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/goloop/ai"
)

func askToSearch(h ...ai.Hosted) *ai.Request {
	return &ai.Request{
		Model:    "the-model",
		Messages: []ai.Message{ai.UserText("What happened today?")},
		Hosted:   h,
	}
}

// The endpoint split is the whole design of this driver's hosted support, so
// it is pinned: nothing hosted goes where it always went, and hosted goes to
// the only endpoint that can run it.
func TestHostedPicksTheEndpoint(t *testing.T) {
	tests := []struct {
		name string
		req  *ai.Request
		want string
	}{
		{"plain", askToSearch(), "/chat/completions"},
		{"hosted", askToSearch(ai.Hosted{Kind: ai.HostedWebSearch}), "/responses"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var path string
			srv := httptest.NewServer(http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					path = r.URL.Path
					_, _ = w.Write([]byte(`{"status":"completed","output":[]}`))
				}))
			defer srv.Close()

			c := New("k", WithBaseURL(srv.URL))
			if _, err := c.Generate(context.Background(), tt.req); err != nil {
				t.Fatal(err)
			}
			if path != tt.want {
				t.Errorf("posted to %q, want %q", path, tt.want)
			}
		})
	}
}

// The same split has to hold for a stream, or a caller could search in one
// call and not the other.
func TestHostedStreamPicksTheEndpoint(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			path = r.URL.Path
			w.Header().Set("content-type", "text/event-stream")
			_, _ = w.Write([]byte("data: {\"type\":\"response.completed\"," +
				"\"response\":{\"usage\":{\"input_tokens\":1,\"output_tokens\":2}}}\n\n"))
		}))
	defer srv.Close()

	c := New("k", WithBaseURL(srv.URL))
	for _, err := range c.Stream(context.Background(),
		askToSearch(ai.Hosted{Kind: ai.HostedWebSearch})) {
		if err != nil {
			t.Fatal(err)
		}
	}
	if path != "/responses" {
		t.Errorf("streamed from %q, want /responses", path)
	}
}

func TestHostedWebSearchReachesTheRequest(t *testing.T) {
	c := New("k")

	rr, err := c.responsesRequest(askToSearch(ai.Hosted{
		Kind: ai.HostedWebSearch,
		Web: &ai.HostedWeb{
			AllowDomains: []string{"example.org"},
			Region:       "UA",
		},
	}), false)
	if err != nil {
		t.Fatal(err)
	}

	if len(rr.Tools) != 1 || rr.Tools[0].Type != webSearchTool {
		t.Fatalf("Tools = %+v, want the search tool", rr.Tools)
	}
	got := rr.Tools[0]
	if got.Filters == nil || len(got.Filters.AllowedDomains) != 1 {
		t.Errorf("Filters = %+v", got.Filters)
	}
	if got.UserLocation == nil || got.UserLocation.Country != "UA" {
		t.Errorf("UserLocation = %+v", got.UserLocation)
	}
}

// This provider filters by allowed domains only, and has no use limit. A
// caller who asked for either is told, not quietly given a wider search.
func TestHostedRejectsSettingsThisEndpointHasNoPlaceFor(t *testing.T) {
	c := New("k")
	tests := []struct {
		name string
		web  ai.HostedWeb
	}{
		{"use limit", ai.HostedWeb{MaxUses: 2}},
		{"blocked domains", ai.HostedWeb{BlockDomains: []string{"b.example"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			web := tt.web
			_, err := c.responsesRequest(askToSearch(ai.Hosted{
				Kind: ai.HostedWebSearch,
				Web:  &web,
			}), false)
			if !errors.Is(err, ai.ErrNoHosted) {
				t.Errorf("responsesRequest() error = %v, want ErrNoHosted", err)
			}
		})
	}
}

// Stop sequences have no field on this endpoint, and silently dropping them
// would move where the answer ends.
func TestHostedRejectsStopSequences(t *testing.T) {
	req := askToSearch(ai.Hosted{Kind: ai.HostedWebSearch})
	req.Stop = []string{"###"}

	if _, err := New("k").responsesRequest(req, false); !errors.Is(err, ai.ErrNoHosted) {
		t.Errorf("responsesRequest() error = %v, want ErrNoHosted", err)
	}
}

// A schema travels to this endpoint in a different shape than to the other
// one: the same fields, one level flatter.
func TestFormatIsReshapedForTheResponsesEndpoint(t *testing.T) {
	req := askToSearch(ai.Hosted{Kind: ai.HostedWebSearch})
	req.Format = &ai.Format{
		Type:   ai.FormatJSONSchema,
		Name:   "seo",
		Schema: json.RawMessage(`{"type":"object"}`),
		Strict: true,
	}

	rr, err := New("k").responsesRequest(req, false)
	if err != nil {
		t.Fatal(err)
	}
	if rr.Text == nil {
		t.Fatal("no text format was sent")
	}

	var got struct {
		Type   string          `json:"type"`
		Name   string          `json:"name"`
		Schema json.RawMessage `json:"schema"`
		Strict bool            `json:"strict"`
	}
	if err := json.Unmarshal(rr.Text.Format, &got); err != nil {
		t.Fatal(err)
	}
	if got.Type != "json_schema" || got.Name != "seo" || !got.Strict {
		t.Errorf("format = %+v", got)
	}
	if string(got.Schema) != `{"type":"object"}` {
		t.Errorf("schema = %s", got.Schema)
	}
}

// A conversation reaches the other endpoint in its own shape: tool calls are
// items of the input list rather than fields of a message.
func TestConversationIsRenderedForTheResponsesEndpoint(t *testing.T) {
	req := &ai.Request{
		Model:  "the-model",
		System: "Be terse.",
		Messages: []ai.Message{
			ai.UserText("hi"),
			{Role: ai.RoleAssistant, Parts: []ai.Part{
				ai.Text{Text: "one moment"},
				ai.ToolUse{ID: "call_1", Name: "lookup",
					Input: json.RawMessage(`{"q":"x"}`)},
			}},
			{Role: ai.RoleTool, Parts: []ai.Part{
				ai.ToolResult{ID: "call_1", Content: "42"},
			}},
		},
		Hosted: []ai.Hosted{{Kind: ai.HostedWebSearch}},
	}

	rr, err := New("k").responsesRequest(req, false)
	if err != nil {
		t.Fatal(err)
	}
	if rr.Instructions != "Be terse." {
		t.Errorf("Instructions = %q", rr.Instructions)
	}

	body, err := json.Marshal(rr.Input)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"input_text"`, `"output_text"`,
		`"function_call"`, `"call_id":"call_1"`,
		`"function_call_output"`, `"output":"42"`,
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("input does not carry %s: %s", want, body)
		}
	}
}

// searchReply is what this endpoint returns for a grounded answer: the search
// it ran as its own output item, and the sources as annotations on the text.
const searchReply = `{
  "model": "the-model",
  "status": "completed",
  "output": [
    {"type": "web_search_call", "id": "ws_1", "status": "completed"},
    {"type": "message", "role": "assistant", "content": [
      {"type": "output_text", "text": "It is warm today.",
       "annotations": [{"type": "url_citation", "url": "https://example.org/a",
                        "title": "A", "start_index": 0, "end_index": 17}]}
    ]}
  ],
  "usage": {"input_tokens": 10, "output_tokens": 5}
}`

func TestGenerateReportsSearchAndCitations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(searchReply))
		}))
	defer srv.Close()

	c := New("k", WithBaseURL(srv.URL))
	resp, err := c.Generate(context.Background(),
		askToSearch(ai.Hosted{Kind: ai.HostedWebSearch}))
	if err != nil {
		t.Fatal(err)
	}

	// The search the provider ran must not look like a call to answer.
	if calls := resp.ToolCalls(); len(calls) != 0 {
		t.Errorf("ToolCalls() = %+v, want none", calls)
	}

	want := ai.HostedReport{
		Kind: ai.HostedWebSearch, Mode: ai.HostedNative, Calls: 1,
	}
	if len(resp.Hosted) != 1 || resp.Hosted[0] != want {
		t.Errorf("Hosted = %+v, want %+v", resp.Hosted, want)
	}

	cs := resp.Citations()
	if len(cs) != 1 || cs[0].URL != "https://example.org/a" {
		t.Fatalf("Citations() = %+v", cs)
	}

	// The answer is ASCII, so the reported indices mean the same number in
	// every unit they could be counted in, and the range is kept.
	if cs[0].StartByte != 0 || cs[0].EndByte != 17 {
		t.Errorf("range = %d..%d, want 0..17", cs[0].StartByte, cs[0].EndByte)
	}

	// The endpoint says "completed"; the package says what chat completions
	// would have said, so a caller need not know which one answered.
	if resp.StopReason != "stop" {
		t.Errorf("StopReason = %q, want stop", resp.StopReason)
	}
}

// Outside ASCII the reported indices cannot be trusted to be byte offsets, and
// a range that cuts mid-character is worse than none.
func TestCitationRangeIsDroppedOnNonASCIIText(t *testing.T) {
	cs := convAnnotations("Сьогодні тепло.", []ResponseAnnotation{{
		Type:       "url_citation",
		URL:        "https://example.org",
		StartIndex: 0,
		EndIndex:   8,
	}})
	if len(cs) != 1 {
		t.Fatalf("citations = %+v, want one", cs)
	}
	if cs[0].StartByte != 0 || cs[0].EndByte != 0 {
		t.Errorf("range = %d..%d, want it dropped",
			cs[0].StartByte, cs[0].EndByte)
	}
	if cs[0].URL != "https://example.org" {
		t.Errorf("the source itself was lost: %+v", cs[0])
	}
}

func TestResponsesStopReasonSpeaksTheChatVocabulary(t *testing.T) {
	tests := []struct {
		name  string
		rr    ResponsesResponse
		calls bool
		want  string
	}{
		{"finished", ResponsesResponse{Status: "completed"}, false, "stop"},
		{"tool call", ResponsesResponse{Status: "completed"}, true, "tool_calls"},
		{
			"ran out of tokens",
			ResponsesResponse{
				Status:            "incomplete",
				IncompleteDetails: &IncompleteDetails{Reason: "max_output_tokens"},
			},
			false, "length",
		},
		{
			"filtered",
			ResponsesResponse{
				Status:            "incomplete",
				IncompleteDetails: &IncompleteDetails{Reason: "content_filter"},
			},
			false, "content_filter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := tt.rr
			if got := responsesStopReason(&rr, tt.calls); got != tt.want {
				t.Errorf("responsesStopReason() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerateReportsASkippedSearch(t *testing.T) {
	const reply = `{"model":"m","status":"completed","output":[
	  {"type":"message","role":"assistant",
	   "content":[{"type":"output_text","text":"I already know."}]}],
	  "usage":{"input_tokens":1,"output_tokens":1}}`

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(reply))
		}))
	defer srv.Close()

	c := New("k", WithBaseURL(srv.URL))

	resp, err := c.Generate(context.Background(),
		askToSearch(ai.Hosted{Kind: ai.HostedWebSearch}))
	if err != nil {
		t.Fatal(err)
	}
	want := ai.HostedReport{Kind: ai.HostedWebSearch, Mode: ai.HostedSkipped}
	if len(resp.Hosted) != 1 || resp.Hosted[0] != want {
		t.Errorf("Hosted = %+v, want %+v", resp.Hosted, want)
	}

	_, err = c.Generate(context.Background(), askToSearch(ai.Hosted{
		Kind:   ai.HostedWebSearch,
		Policy: ai.HostedRequired,
	}))
	if !errors.Is(err, ai.ErrHostedRequired) {
		t.Errorf("Generate() error = %v, want ErrHostedRequired", err)
	}
}

func TestStreamCarriesCitationsAndReport(t *testing.T) {
	const events = `data: {"type":"response.output_item.added","item":{"type":"web_search_call","id":"ws_1"}}

data: {"type":"response.output_text.delta","delta":"It is warm"}

data: {"type":"response.output_text.annotation.added","annotation":{"type":"url_citation","url":"https://example.org/a","title":"A","start_index":0,"end_index":10}}

data: {"type":"response.completed","response":{"status":"completed","output":[{"type":"web_search_call","id":"ws_1"}],"usage":{"input_tokens":3,"output_tokens":4}}}

`

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("content-type", "text/event-stream")
			_, _ = w.Write([]byte(events))
		}))
	defer srv.Close()

	c := New("k", WithBaseURL(srv.URL))

	var text strings.Builder
	var cites []ai.Citation
	var done ai.Chunk
	for chunk, err := range c.Stream(context.Background(),
		askToSearch(ai.Hosted{Kind: ai.HostedWebSearch})) {
		if err != nil {
			t.Fatal(err)
		}
		text.WriteString(chunk.Text)
		cites = append(cites, chunk.Citations...)
		if chunk.Done {
			done = chunk
		}
	}

	if text.String() != "It is warm" {
		t.Errorf("text = %q", text.String())
	}
	if len(cites) != 1 || cites[0].URL != "https://example.org/a" {
		t.Fatalf("citations = %+v", cites)
	}
	if cites[0].StartByte != 0 || cites[0].EndByte != 0 {
		t.Errorf("a streamed citation carries a range: %+v", cites[0])
	}

	want := ai.HostedReport{
		Kind: ai.HostedWebSearch, Mode: ai.HostedNative, Calls: 1,
	}
	if len(done.Hosted) != 1 || done.Hosted[0] != want {
		t.Errorf("Hosted on the final chunk = %+v, want %+v", done.Hosted, want)
	}
	if done.Usage == nil || done.Usage.OutputTokens != 4 {
		t.Errorf("Usage = %+v", done.Usage)
	}
}

// A tool loop must work the same over the responses endpoint as over the other
// one: the caller's own calls still arrive, with their arguments.
func TestStreamStillDeliversTheCallersToolCalls(t *testing.T) {
	const events = `data: {"type":"response.output_item.added","item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"lookup"}}

data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","delta":"{\"q\":"}

data: {"type":"response.function_call_arguments.done","item_id":"fc_1","arguments":"{\"q\":\"x\"}"}

data: {"type":"response.completed","response":{"status":"completed","output":[],"usage":{}}}

`

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("content-type", "text/event-stream")
			_, _ = w.Write([]byte(events))
		}))
	defer srv.Close()

	c := New("k", WithBaseURL(srv.URL))

	var calls []ai.ToolUse
	for chunk, err := range c.Stream(context.Background(),
		askToSearch(ai.Hosted{Kind: ai.HostedWebSearch})) {
		if err != nil {
			t.Fatal(err)
		}
		if chunk.ToolCall != nil {
			calls = append(calls, *chunk.ToolCall)
		}
	}

	if len(calls) != 1 {
		t.Fatalf("tool calls = %+v, want one", calls)
	}
	if calls[0].ID != "call_1" || calls[0].Name != "lookup" {
		t.Errorf("call = %+v", calls[0])
	}
	if string(calls[0].Input) != `{"q":"x"}` {
		t.Errorf("arguments = %s", calls[0].Input)
	}
}
