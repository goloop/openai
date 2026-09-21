package openai

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/goloop/ai"
)

// streamOf serves the given SSE lines and runs Stream over them, returning
// the tool calls handed out, whether Done was reported and the final error.
func streamOf(t *testing.T, events []string) (calls []*ai.ToolUse, doneSeen bool, err error) {
	t.Helper()
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		for _, line := range events {
			io.WriteString(w, line+"\n")
		}
	})
	defer done()

	for chunk, e := range c.Stream(context.Background(), &ai.Request{
		Model: "m", Messages: []ai.Message{ai.UserText("hi")},
	}) {
		if e != nil {
			return calls, doneSeen, e
		}
		if chunk.ToolCall != nil {
			calls = append(calls, chunk.ToolCall)
		}
		if chunk.Done {
			doneSeen = true
		}
	}
	return calls, doneSeen, nil
}

// A stream cut off in the middle of a tool call must not hand out the
// half-built call, and must not report Done: the consumer gets an error
// before anything it could act on.
func TestStreamCutOffKeepsOpenToolCall(t *testing.T) {
	calls, doneSeen, err := streamOf(t, []string{
		`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call1",` +
			`"function":{"name":"action","arguments":"{\"x\":"}}]}}]}`, ``,
		// connection ends here - no finish_reason, no [DONE]
	})
	if len(calls) != 0 {
		t.Errorf("got %d tool calls from a cut-off stream, want none", len(calls))
	}
	if doneSeen {
		t.Error("a cut-off stream reported Done")
	}
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("err = %v, want ErrUnexpectedEOF", err)
	}
}

// A stream cut off after only a text delta is an error too, not a success
// with a shorter answer.
func TestStreamCutOffAfterText(t *testing.T) {
	_, doneSeen, err := streamOf(t, []string{
		`data: {"choices":[{"index":0,"delta":{"content":"Hel"}}]}`, ``,
	})
	if doneSeen {
		t.Error("a cut-off stream reported Done")
	}
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("err = %v, want ErrUnexpectedEOF", err)
	}
}

// A finish_reason marks the answer complete even when the [DONE] sentinel
// that should follow it never arrives.
func TestStreamFinishReasonWithoutDoneIsComplete(t *testing.T) {
	_, doneSeen, err := streamOf(t, []string{
		`data: {"choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":"stop"}]}`, ``,
	})
	if err != nil {
		t.Fatalf("err = %v, want none", err)
	}
	if !doneSeen {
		t.Error("a stream with a finish_reason did not report Done")
	}
}

// Tool arguments that are not valid JSON are an error before any call is
// handed out, even when the stream ends properly.
func TestStreamInvalidToolArgumentsAreAnError(t *testing.T) {
	calls, _, err := streamOf(t, []string{
		`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call1",` +
			`"function":{"name":"action","arguments":"{\"x\":"}}]},"finish_reason":"tool_calls"}]}`, ``,
		`data: [DONE]`, ``,
	})
	if len(calls) != 0 {
		t.Errorf("got %d tool calls with invalid arguments, want none", len(calls))
	}
	if err == nil {
		t.Error("invalid tool arguments were not reported")
	}
}
