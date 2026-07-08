package openai

import (
	"context"
	"encoding/json"
	"net/http"
)

// File describes an uploaded file.
type File struct {
	ID        string `json:"id"`
	Object    string `json:"object"`
	Bytes     int64  `json:"bytes"`
	CreatedAt int64  `json:"created_at"`
	Filename  string `json:"filename"`
	Purpose   string `json:"purpose"`
}

// UploadFile uploads a file for the given purpose (for example "batch" or
// "fine-tune").
func (c *Client) UploadFile(
	ctx context.Context,
	filename string,
	data []byte,
	purpose string,
) (*File, error) {
	body, err := c.postMultipart(ctx, "/files",
		map[string]string{"purpose": purpose},
		formFile{field: "file", filename: filename, data: data})
	if err != nil {
		return nil, err
	}
	var f File
	if err := json.Unmarshal(body, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// Files lists uploaded files.
func (c *Client) Files(ctx context.Context) ([]File, error) {
	var out struct {
		Data []File `json:"data"`
	}
	if err := c.getJSON(ctx, "/files", &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// GetFile returns a single file's metadata.
func (c *Client) GetFile(ctx context.Context, id string) (*File, error) {
	var f File
	if err := c.getJSON(ctx, "/files/"+id, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// FileContent downloads a file's contents.
func (c *Client) FileContent(ctx context.Context, id string) ([]byte, error) {
	data, status, err := c.send(ctx, http.MethodGet, "/files/"+id+"/content", nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, parseError(status, data)
	}
	return data, nil
}

// DeleteFile deletes an uploaded file.
func (c *Client) DeleteFile(ctx context.Context, id string) error {
	data, status, err := c.send(ctx, http.MethodDelete, "/files/"+id, nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return parseError(status, data)
	}
	return nil
}
