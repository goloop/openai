package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

// Batch is the state of a batch job.
type Batch struct {
	ID            string      `json:"id"`
	Object        string      `json:"object"`
	Endpoint      string      `json:"endpoint"`
	Status        string      `json:"status"`
	InputFileID   string      `json:"input_file_id"`
	OutputFileID  string      `json:"output_file_id"`
	ErrorFileID   string      `json:"error_file_id"`
	CreatedAt     int64       `json:"created_at"`
	RequestCounts BatchCounts `json:"request_counts"`
}

// BatchCounts breaks down how many requests are in each state.
type BatchCounts struct {
	Total     int `json:"total"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

// CreateBatch starts a batch that runs the requests in the uploaded input file
// against endpoint (for example "/v1/chat/completions"). completionWindow is
// usually "24h". Upload the JSONL input with UploadFile(purpose "batch") first.
func (c *Client) CreateBatch(
	ctx context.Context,
	inputFileID, endpoint, completionWindow string,
) (*Batch, error) {
	if completionWindow == "" {
		completionWindow = "24h"
	}
	req := map[string]string{
		"input_file_id":     inputFileID,
		"endpoint":          endpoint,
		"completion_window": completionWindow,
	}
	var b Batch
	if err := c.postJSON(ctx, "/batches", req, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// GetBatch returns the current state of a batch.
func (c *Client) GetBatch(ctx context.Context, id string) (*Batch, error) {
	var b Batch
	if err := c.getJSON(ctx, "/batches/"+url.PathEscape(id), &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// ListBatches lists batches, most recent first.
func (c *Client) ListBatches(ctx context.Context) ([]Batch, error) {
	var out struct {
		Data []Batch `json:"data"`
	}
	if err := c.getJSON(ctx, "/batches", &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// CancelBatch requests cancellation of a batch in progress.
func (c *Client) CancelBatch(ctx context.Context, id string) (*Batch, error) {
	data, status, err := c.send(ctx, http.MethodPost,
		"/batches/"+url.PathEscape(id)+"/cancel", nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, parseError(status, data)
	}
	var b Batch
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	return &b, nil
}
