package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
)

// headers returns the headers common to every OpenAI request.
func (c *Client) headers() http.Header {
	h := http.Header{}
	h.Set("authorization", "Bearer "+c.opts.APIKey)
	h.Set("content-type", "application/json")
	if c.orgID != "" {
		h.Set("openai-organization", c.orgID)
	}
	if c.project != "" {
		h.Set("openai-project", c.project)
	}
	return h
}

// send performs a request against a path under the base URL and returns the
// response body and status code.
func (c *Client) send(
	ctx context.Context,
	method, path string,
	body []byte,
) ([]byte, int, error) {
	resp, err := c.opts.Do(ctx, method, c.opts.BaseURL+path, body, c.headers())
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return data, resp.StatusCode, nil
}

// postJSON marshals in, POSTs it to path and unmarshals the response into out.
func (c *Client) postJSON(ctx context.Context, path string, in, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}
	data, status, err := c.send(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return parseError(status, data)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}

// getJSON GETs path and unmarshals the response into out.
func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	data, status, err := c.send(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return parseError(status, data)
	}
	return json.Unmarshal(data, out)
}

// formFile is one file field in a multipart request.
type formFile struct {
	field    string
	filename string
	data     []byte
}

// postMultipart sends a multipart/form-data POST with the given fields and
// files, returning the raw response body.
func (c *Client) postMultipart(
	ctx context.Context,
	path string,
	fields map[string]string,
	files ...formFile,
) ([]byte, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			return nil, err
		}
	}
	for _, f := range files {
		w, err := mw.CreateFormFile(f.field, f.filename)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(f.data); err != nil {
			return nil, err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}

	h := c.headers()
	h.Set("content-type", mw.FormDataContentType())
	resp, err := c.opts.Do(ctx, http.MethodPost, c.opts.BaseURL+path, buf.Bytes(), h)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp.StatusCode, data)
	}
	return data, nil
}
