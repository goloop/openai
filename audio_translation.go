package openai

import "os"

// AudioTranslationRequest represents a request to the OpenAI Translation API.
type AudioTranslationRequest struct {
	// The audio file to translate, in one of these formats: mp3, mp4,
	// mpeg, mpga, m4a, wav, or webm. This is required.
	File *os.File `json:"file"`

	// The model ID to use for the request. Only whisper-1 is currently
	// available. This is required.
	Model string `json:"model"`

	// An optional text to guide the model's style or continue a previous
	// audio segment. The prompt should be in English.
	Prompt string `json:"prompt,omitempty"`

	// The format of the transcript output. Options include: json, text,
	// srt, verbose_json, or vtt. Defaults to json if not specified.
	ResponseFormat string `json:"response_format,omitempty"`

	// The sampling temperature, between 0 and 1. Higher values like 0.8
	// will make the output more random, while lower values like 0.2 will
	// make it more focused and deterministic. If set to 0, the model will
	// use log probability to automatically increase the temperature until
	// certain thresholds are hit.
	Temperature float64 `json:"temperature,omitempty"`
}

// AudioTranslationResponse represents the response from the OpenAI
// Translation API.
type AudioTranslationResponse struct {
	// The translated text.
	Text string `json:"text"`
}

// Error returns an error if the request is invalid.
func (r *AudioTranslationRequest) Error() error {
	if r.File == nil {
		return ErrFileRequired
	}

	if r.Model == "" {
		return ErrModelRequired
	}

	return nil
}

// OpenAudioFile reads an audio file from the provided path and assigns
// the *os.File value to the File field of the request.
func (r *AudioTranslationRequest) OpenAudioFile(path string) error {
	r.CloseAudioFile()
	file, err := os.Open(path)
	if err != nil {
		return err
	}

	// Assign the opened file to the File field of the request.
	r.File = file
	return nil
}

// CloseAudioFile closes the audio file associated with the request.
func (r *AudioTranslationRequest) CloseAudioFile() {
	if r.File != nil {
		r.File.Close()
	}
}

// Flush closes the files descriptors associated with the request.
func (r *AudioTranslationRequest) Flush() {
	r.CloseAudioFile()
}
