package openai

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// Image model identifiers. Any model string is accepted.
const (
	ModelGPTImage1 = "gpt-image-1"
	ModelDallE3    = "dall-e-3"
	ModelDallE2    = "dall-e-2"
)

// The values ImageRequest.ResponseFormat accepts.
const (
	ImageFormatURL     = "url"
	ImageFormatB64JSON = "b64_json"
)

// Errors reported for an image request or its result.
var (
	// ErrNoImageRequest is returned by GenerateImage for a nil request.
	ErrNoImageRequest = errors.New("openai: image request is nil")

	// ErrImageFormat is returned before the request is sent, when the chosen
	// model cannot return images in the format asked for.
	ErrImageFormat = errors.New("openai: model cannot return that image format")

	// ErrNoImageBytes is returned by ImageData.Bytes for an image the provider
	// returned as a URL. Fetch the URL, or ask for ImageFormatB64JSON.
	ErrNoImageBytes = errors.New("openai: image was returned as a URL")
)

// ImageRequest is an image generation request.
type ImageRequest struct {
	Model   string `json:"model,omitempty"`
	Prompt  string `json:"prompt"`
	N       int    `json:"n,omitempty"`
	Size    string `json:"size,omitempty"`
	Quality string `json:"quality,omitempty"`
	Style   string `json:"style,omitempty"`

	// ResponseFormat is ImageFormatURL or ImageFormatB64JSON. Models that
	// always answer with base64 do not accept the field at all; see
	// [Client.GenerateImage] for what happens then.
	ResponseFormat string `json:"response_format,omitempty"`

	// The fields below belong to the gpt-image family only. dall-e models do
	// not accept them, so setting one on a dall-e model is [ErrImageFormat]
	// before the request is sent - the same fail-early treatment
	// ResponseFormat gets - rather than a provider rejection with a less
	// helpful message. Empty means the provider's own default.
	//
	// Background is "transparent", "opaque" or "auto".
	Background string `json:"background,omitempty"`

	// OutputFormat is "png" (the default), "jpeg" or "webp". webp and jpeg
	// are smaller on disk and quicker to return than png.
	OutputFormat string `json:"output_format,omitempty"`

	// OutputCompression is the compression level (0-100) for jpeg and webp.
	// It is a pointer because 0 is a meaningful value - no compression - and
	// has to be distinguishable from "not set".
	OutputCompression *int `json:"output_compression,omitempty"`

	// Moderation is "low" or "auto".
	Moderation string `json:"moderation,omitempty"`

	User string `json:"user,omitempty"`
}

// ImageResponse is an image generation response.
type ImageResponse struct {
	Created int64       `json:"created"`
	Data    []ImageData `json:"data"`

	// Usage reports the tokens an image request consumed. gpt-image returns
	// it; dall-e does not, so it is a pointer - a nil Usage is "the provider
	// did not report it", distinct from a zero count. Image generation is
	// billed apart from text, and this is the only place its cost shows up.
	Usage *ImageUsage `json:"usage,omitempty"`
}

// ImageUsage reports the tokens an image request consumed.
type ImageUsage struct {
	TotalTokens        int                      `json:"total_tokens"`
	InputTokens        int                      `json:"input_tokens"`
	OutputTokens       int                      `json:"output_tokens"`
	InputTokensDetails *ImageInputTokensDetails `json:"input_tokens_details,omitempty"`
}

// ImageInputTokensDetails splits the input tokens between the text prompt and
// any input images, which is where the cost of an edit or variation lands.
type ImageInputTokensDetails struct {
	TextTokens  int `json:"text_tokens"`
	ImageTokens int `json:"image_tokens"`
}

// ImageData is one generated image, as a URL or base64 JSON.
type ImageData struct {
	URL           string `json:"url"`
	B64JSON       string `json:"b64_json"`
	RevisedPrompt string `json:"revised_prompt"`
}

// Bytes returns the decoded image. It reads what the provider put inline and
// does no I/O: an image delivered as a URL returns [ErrNoImageBytes], because
// fetching it is a network call with the caller's own timeouts, redirects and
// proxy rules, not something a field accessor should decide.
func (d ImageData) Bytes() ([]byte, error) {
	if d.B64JSON == "" {
		if d.URL != "" {
			return nil, fmt.Errorf("%w: %s", ErrNoImageBytes, d.URL)
		}
		return nil, ErrNoImageBytes
	}
	return base64.StdEncoding.DecodeString(d.B64JSON)
}

// GenerateImage creates images from a text prompt.
//
// The request is fitted to the model first. The gpt-image family always
// answers with base64 and rejects response_format outright, so for those
// models the field is dropped when it asks for base64, and asking them for a
// URL is reported as [ErrImageFormat] before anything is sent - rather than
// becoming a successful reply whose URL is empty. Every other model receives
// the request unchanged.
//
// Read the image with [ImageData.Bytes].
func (c *Client) GenerateImage(ctx context.Context, req *ImageRequest) (*ImageResponse, error) {
	payload, err := imagePayload(req)
	if err != nil {
		return nil, err
	}

	var out ImageResponse
	if err := c.postJSON(ctx, "/images/generations", payload, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// imagePayload returns the request as it should go on the wire, leaving the
// caller's own value untouched.
func imagePayload(req *ImageRequest) (*ImageRequest, error) {
	if req == nil {
		return nil, ErrNoImageRequest
	}

	// The gpt-image-only fields are rejected on a model that cannot accept
	// them, before the network. Silently dropping them would turn a caller's
	// webp request into a png without a word; refusing says why.
	if !inlineOnly(req.Model) {
		if field := gptImageOnlyField(req); field != "" {
			return nil, fmt.Errorf("%w: %s does not accept %s",
				ErrImageFormat, modelName(req.Model), field)
		}
	}

	if !inlineOnly(req.Model) || req.ResponseFormat == "" {
		return req, nil
	}
	if req.ResponseFormat != ImageFormatB64JSON {
		return nil, fmt.Errorf("%w: %s returns %s, not %q",
			ErrImageFormat, req.Model, ImageFormatB64JSON, req.ResponseFormat)
	}

	out := *req
	out.ResponseFormat = ""
	return &out, nil
}

// gptImageOnlyField returns the name of a gpt-image-only field the request
// sets, or "" if it sets none. It exists so the refusal above can name the
// exact field the caller has to remove.
func gptImageOnlyField(req *ImageRequest) string {
	switch {
	case req.Background != "":
		return "background"
	case req.OutputFormat != "":
		return "output_format"
	case req.OutputCompression != nil:
		return "output_compression"
	case req.Moderation != "":
		return "moderation"
	default:
		return ""
	}
}

// modelName renders the model for an error message, naming the default when the
// caller left it empty.
func modelName(model string) string {
	if model == "" {
		return "the default model"
	}
	return model
}

// inlineOnly reports whether a model always returns image bytes inline and
// refuses response_format.
//
// It matches the family and its versions rather than a list of exact names: a
// list would be stale the week a snapshot ships, while a bare prefix would also
// claim an unrelated model whose name merely starts the same way. The rule
// fails safe - a model matched by mistake is only refused a URL it probably
// could not produce either - and the day the family gains URL support this is
// one line to relax.
func inlineOnly(model string) bool {
	return model == imageFamily || strings.HasPrefix(model, imageFamily+"-")
}

// imageFamily is the model family that answers only with inline base64.
const imageFamily = "gpt-image"
