package openai

import "github.com/goloop/g"

// Check if ImageGenerationRequest implements Requester interface.
var _ Requester = (*ImageGenerationRequest)(nil)

// ImageGenerationRequest represents the structure
// of a request to the OpenAI API.
type ImageGenerationRequest struct {
	// Prompt is a text description of the desired image(s).
	Prompt string `json:"prompt"`

	// N is the number of images to generate. Optional.
	N int `json:"n,omitempty"`

	// Size of the generated images. Optional.
	Size string `json:"size,omitempty"`

	// ResponseFormat is the format in which the generated images
	// are returned. Optional.
	ResponseFormat string `json:"response_format,omitempty"`

	// User is a unique identifier representing the end-user. Optional.
	User string `json:"user,omitempty"`
}

// ImageGenerationData represents the structure
// of an image in the response data.
type ImageGenerationData struct {
	// URL is the address where the generated image can be accessed.
	URL string `json:"url,omitempty"`

	// B64 is the base64 encoded image data.
	Base64 string `json:"b64_json,omitempty"`
}

// ImageGenerationResponse represents the structure
// of a response from the OpenAI API.
type ImageGenerationResponse struct {
	// Created is the timestamp when the image(s) was generated.
	Created int `json:"created"`

	// Data is an array of generated image data.
	Data []ImageGenerationData `json:"data"`

	// Is the number of parallel tasks to use when saving images.
	parallelTasks int
}

// Error returns an error if the request is invalid.
func (r *ImageGenerationRequest) Error() error {
	if r.Prompt == "" {
		return ErrPromptRequired
	}

	if !g.In(r.ResponseFormat, validImageResponseFormats...) {
		return ErrInvalidResponseFormat
	}

	if !g.In(r.Size, validImageSizes...) {
		return ErrInvalidSize
	}

	return nil
}

// Flush does nothing.
// It is here to satisfy the Requester interface.
func (r *ImageGenerationRequest) Flush() {}

func (r *ImageGenerationResponse) Save(path string) error {
	if len(r.Data) == 0 {
		return nil
	}

	if r.Data[0].URL != "" {
		items := make([]string, len(r.Data))
		for i, data := range r.Data {
			items[i] = data.URL
		}

		return saveByURL(path, g.Value(r.parallelTasks, parallelTasks), items)
	}

	if r.Data[0].Base64 != "" {
		items := make([]string, len(r.Data))
		for i, data := range r.Data {
			items[i] = data.Base64
		}

		return saveByBase64(path, g.Value(r.parallelTasks, parallelTasks), items)
	}

	return nil
}
