package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

// maxJSONBytes caps how much of a JSON response body is read into memory, so a
// malformed or hostile server cannot exhaust memory with an unbounded stream.
const maxJSONBytes = 64 << 20 // 64 MiB

// maxBinaryBytes is the in-memory ceiling for genuinely binary bodies (file
// downloads, synthesized speech). These can be larger than a JSON reply, so
// the ceiling is higher, but it stays bounded so a runaway body cannot exhaust
// memory. Callers that need a bigger body stream it with FileContentTo or
// SpeechTo instead of buffering it here.
const maxBinaryBytes = 128 << 20 // 128 MiB

// readLimited reads up to limit bytes from r, returning an error if the body
// exceeds that ceiling rather than silently truncating it.
func readLimited(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("openai: response body exceeds %d bytes", limit)
	}
	return data, nil
}

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
// response body (read under the JSON ceiling) and status code.
func (c *Client) send(
	ctx context.Context,
	method, path string,
	body []byte,
) ([]byte, int, error) {
	return c.sendLimited(ctx, method, path, body, maxJSONBytes)
}

// sendLimited is send with an explicit read ceiling, for endpoints whose
// bodies are binary and can be larger than a JSON reply.
func (c *Client) sendLimited(
	ctx context.Context,
	method, path string,
	body []byte,
	limit int64,
) ([]byte, int, error) {
	resp, err := c.opts.Do(ctx, method, c.opts.BaseURL+path, body, c.headers())
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	data, err := readLimited(resp.Body, limit)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return data, resp.StatusCode, nil
}

// sendTo performs a request and streams a successful response body to w,
// copying it without buffering the whole body in memory. On a non-success
// status it reads the bounded error body and returns an *ai.APIError.
func (c *Client) sendTo(
	ctx context.Context,
	method, path string,
	body []byte,
	w io.Writer,
) error {
	resp, err := c.opts.Do(ctx, method, c.opts.BaseURL+path, body, c.headers())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := readLimited(resp.Body, maxJSONBytes)
		return parseError(resp.StatusCode, data)
	}
	_, err = io.Copy(w, resp.Body)
	return err
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

	data, err := readLimited(resp.Body, maxJSONBytes)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp.StatusCode, data)
	}
	return data, nil
}
