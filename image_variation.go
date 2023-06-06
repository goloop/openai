package openai

import (
	"os"

	"github.com/goloop/g"
)

// Check if ImageVariationRequest implements Requester interface.
var _ Requester = (*ImageVariationRequest)(nil)

// ImageVariationRequest represents a request to the OpenAI Image Variation API.
// The image field is required and must be a valid PNG file, less than 4MB,
// and square. The optional fields include n (number of images to generate,
// default is 1), size (the size of the generated images, default is 1024x1024),
// response_format (the format in which the images are returned, default is url),
// and user (a unique identifier representing your end-user).
type ImageVariationRequest struct {
	Image          *os.File `json:"image"`                     // Base64-encoded PNG file
	N              int      `json:"n,omitempty"`               // Number of images to generate
	Size           string   `json:"size,omitempty"`            // Size of the generated images
	ResponseFormat string   `json:"response_format,omitempty"` // Format of the returned images
	User           string   `json:"user,omitempty"`            // Unique identifier of the end-user
}

type ImageVariationData struct {
	// The URL where the generated image can be found.
	URL string `json:"url"`

	// B64 is the base64 encoded image data.
	Base64 string `json:"b64_json,omitempty"`
}

// ImageVariationResponse represents a response from the OpenAI Image
// Variation API. The created field represents the timestamp of creation,
// and the data field includes the generated image data.
type ImageVariationResponse struct {
	Created int64                `json:"created"` // timestamp of creation
	Data    []ImageVariationData `json:"data"`    // generated image data

	// Is the number of parallel tasks to use when saving images.
	parallelTasks int
}

func (r *ImageVariationResponse) Save(path string) error {
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

		return saveByBase64(
			path,
			g.Value(r.parallelTasks, parallelTasks),
			items,
		)
	}

	return nil
}

// OpenImageFile reads an image from a file and assigns the *os.File
// value to the Image field of the request.
func (r *ImageVariationRequest) OpenImageFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}

	// Assign the file to the Image field.
	r.Image = file
	return nil
}

func (r *ImageVariationRequest) Error() error {
	if r.Image == nil {
		return ErrImageRequired
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
func (r *ImageVariationRequest) CloseImageFile() {
	if r.Image != nil {
		r.Image.Close()
	}
}

// Flush closes the files descriptors associated with the request.
func (r *ImageVariationRequest) Flush() {
	r.CloseImageFile()
}
