package openai

import (
	"os"

	"github.com/goloop/g"
)

// Check if ImageEditRequest implements Requester interface.
var _ Requester = (*ImageEditRequest)(nil)

// ImageEditRequest represents a request to the OpenAI Image API.
type ImageEditRequest struct {
	Image          *os.File `json:"image"`                     // Base64-encoded PNG file, less than 4MB, and square.
	Mask           *os.File `json:"mask,omitempty"`            // Optional Base64-encoded PNG mask file.
	Prompt         string   `json:"prompt"`                    // Text description of the desired image(s).
	N              int      `json:"n,omitempty"`               // Number of images to generate. Default 1.
	Size           string   `json:"size,omitempty"`            // Size of the generated images. Default 1024x1024.
	ResponseFormat string   `json:"response_format,omitempty"` // Format in which the images are returned. Default url.
	User           string   `json:"user,omitempty"`            // Unique identifier representing the end-user.
}

type ImageEditData struct {
	// The URL where the generated image can be found.
	URL string `json:"url"`

	// B64 is the base64 encoded image data.
	Base64 string `json:"b64_json,omitempty"`
}

// ImageEditResponse represents a response from the OpenAI Image API.
type ImageEditResponse struct {
	// The time the request was created, in Unix time.
	Created int64 `json:"created"`

	// An array containing the generated images.
	Data []ImageEditData `json:"data"`

	// Is the number of parallel tasks to use when saving images.
	parallelTasks int
}

// OpenImageFile reads an image from a file and assigns the *os.File
// value to the Image field of the request.
func (r *ImageEditRequest) OpenImageFile(path string) error {
	r.CloseImageFile()
	file, err := os.Open(path)
	if err != nil {
		return err
	}

	// Assign the file to the Image field.
	r.Image = file
	return nil
}

// OpenMaskFile reads an image from a file and assigns the *os.File
// value to the Mask field of the request.
func (r *ImageEditRequest) OpenMaskFile(path string) error {
	r.CloseMaskFile()
	file, err := os.Open(path)
	if err != nil {
		return err
	}

	// Assign the file to the Mask field.
	r.Mask = file
	return nil
}

func (r *ImageEditRequest) Error() error {
	if r.Image == nil {
		return ErrImageRequired
	}

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

// CloseImageFile closes the Image file descriptor associated with the request.
func (r *ImageEditRequest) CloseImageFile() {
	if r.Image != nil {
		r.Image.Close()
	}
}

// CloseMaskFile closes the Mask file descriptor associated with the request.
func (r *ImageEditRequest) CloseMaskFile() {
	if r.Image != nil {
		r.Image.Close()
	}
}

// Flush closes the files descriptors associated with the request.
func (r *ImageEditRequest) Flush() {
	r.CloseImageFile()
	r.CloseMaskFile()
}

func (r *ImageEditResponse) Save(path string) error {
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
