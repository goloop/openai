package openai

import (
	"context"
	"encoding/json"
	"net/http"
)

// TranscriptionRequest transcribes or translates an audio file. File holds the
// audio bytes and Filename gives them an extension the API can recognize (for
// example "audio.mp3").
type TranscriptionRequest struct {
	Model    string
	File     []byte
	Filename string
	Language string // transcription only, optional
	Prompt   string
	Format   string // response_format, for example "json" or "text"
}

// Transcribe converts speech in the audio file to text in the same language.
func (c *Client) Transcribe(ctx context.Context, req *TranscriptionRequest) (string, error) {
	return c.audioText(ctx, "/audio/transcriptions", req, true)
}

// Translate converts speech in the audio file to English text.
func (c *Client) Translate(ctx context.Context, req *TranscriptionRequest) (string, error) {
	return c.audioText(ctx, "/audio/translations", req, false)
}

func (c *Client) audioText(
	ctx context.Context,
	path string,
	req *TranscriptionRequest,
	withLanguage bool,
) (string, error) {
	fields := map[string]string{"model": req.Model}
	if withLanguage && req.Language != "" {
		fields["language"] = req.Language
	}
	if req.Prompt != "" {
		fields["prompt"] = req.Prompt
	}
	if req.Format != "" {
		fields["response_format"] = req.Format
	}

	name := req.Filename
	if name == "" {
		name = "audio.mp3"
	}
	data, err := c.postMultipart(ctx, path, fields,
		formFile{field: "file", filename: name, data: req.File})
	if err != nil {
		return "", err
	}

	var out struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		// Non-JSON response formats return the text directly.
		return string(data), nil
	}
	return out.Text, nil
}

// SpeechRequest turns text into speech.
type SpeechRequest struct {
	Model  string  `json:"model"`
	Input  string  `json:"input"`
	Voice  string  `json:"voice"`
	Format string  `json:"response_format,omitempty"`
	Speed  float64 `json:"speed,omitempty"`
}

// Speech synthesizes audio for the given text and returns the raw audio bytes.
func (c *Client) Speech(ctx context.Context, req *SpeechRequest) ([]byte, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	data, status, err := c.send(ctx, http.MethodPost, "/audio/speech", body)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, parseError(status, data)
	}
	return data, nil
}
