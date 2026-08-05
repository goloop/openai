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

	User string `json:"user,omitempty"`
}

// ImageResponse is an image generation response.
type ImageResponse struct {
	Created int64       `json:"created"`
	Data    []ImageData `json:"data"`
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
