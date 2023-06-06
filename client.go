package openai

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/goloop/g"
)

type Clienter interface {
	APIKey() string
	OrgID() string
	Endpoint(...string) string
	ParallelTasks() int
	Context() context.Context
	HTTPHeaders() http.Header
	HTTPClient() *http.Client
}

type Requester interface {
	Error() error
	Flush()
}

// Config represents the OpenAI API configuration.
type Config struct {
	APIKey     string // secret key for authorization
	OrgID      string // unique identifier of the organization
	APIBaseURL string // base URL of OpenAI API

	ParallelTasks  int             // number of parallel requests
	RequestTimeout time.Duration   // maximum duration time for a request
	Context        context.Context // context for requests
	HTTPHeaders    http.Header     // additional HTTP headers for requests
	HTTPClient     *http.Client    // http client for sending requests
}

// Client represents the OpenAI API client.
//
// The client has the same fields as the config, but we don't imitate Config
// because we store private fields, we also don't make a pointer to Config so
// that it is not possible to change the behavior of the Client by changing
// the original Config. The Config fields are copied to the Client settings,
// and some empty fields can take default values.
type Client struct {
	apiKey     string // secret key for authorization
	orgID      string // unique identifier of the organization
	apiBaseURL string // base URL of OpenAI API

	parallelTasks int             // number of parallel requests
	context       context.Context // context for requests
	httpHeaders   http.Header     // additional HTTP headers for requests
	httpClient    *http.Client    // http client for sending requests
}

// Error returns an error if the client has a configuration problem.
func (c *Client) Error() error {
	// APIKey is a required parameter.
	if c.apiKey == "" {
		return ErrNoAPIKey
	}

	// APIBaseURL is a required parameter.
	// Also check if the URL is valid.
	if c.apiBaseURL == "" {
		return ErrNoAPIBaseURL
	} else if _, err := urlBuild(c.apiBaseURL); err != nil {
		return err
	}

	// HTTPClient is a required parameter.
	if c.httpClient == nil {
		return ErrNoHTTPClient
	}

	// Context is a required parameter.
	if c.context == nil {
		return ErrNoContext
	}

	return nil
}

// Configure configures the client.
func (c *Client) Configure(config *Config) {
	c.apiKey = g.Value(config.APIKey, c.apiKey)
	c.orgID = g.Value(config.OrgID, c.orgID)
	c.apiBaseURL = g.Value(config.APIBaseURL, c.apiBaseURL, apiBaseURL)
	c.parallelTasks = g.Value(
		config.ParallelTasks,
		c.parallelTasks,
		parallelTasks,
	)
	c.context = g.Value(config.Context, c.context, context.Background())
	c.httpHeaders = g.Value(config.HTTPHeaders, c.httpHeaders)
	c.httpClient = g.Value(
		config.HTTPClient,
		c.httpClient,
		&http.Client{
			Timeout: requestTimeout,
		},
	)
}

func (c *Client) APIKey() string {
	return c.apiKey
}

func (c *Client) OrgID() string {
	return c.orgID
}

func (c *Client) Endpoint(p ...string) string {
	u, _ := urlBuild(c.apiBaseURL, p...)
	return u
}

func (c *Client) ParallelTasks() int {
	return c.parallelTasks
}

func (c *Client) Context() context.Context {
	return c.context
}

func (c *Client) HTTPHeaders() http.Header {
	return c.httpHeaders
}

func (c *Client) HTTPClient() *http.Client {
	return c.httpClient
}

func (c *Client) Models(models ...string) (ModelsData, error) {
	var wg sync.WaitGroup

	// If no modelIDs are provided, get all models.
	if len(models) == 0 {
		endpoint := c.Endpoint("/models")
		resp := &ModelResponse{}

		req, err := newJsonRequest(c, http.MethodGet, endpoint, nil)
		if err != nil {
			return ModelsData{}, err
		}

		_, err = doRequest(c, req, resp)
		if err != nil {
			return ModelsData{}, err
		}

		return resp.Data, nil
	}

	data := make(ModelsData, len(models))
	errs := make([]error, len(models))

	// Create a buffered channel with a capacity equal
	// to the number of CPU cores.
	sem := make(chan struct{}, c.ParallelTasks())

	for i, modelID := range models {
		wg.Add(1)
		go func(i int, modelID string) {
			// Acquire a "token" from the semaphore.
			sem <- struct{}{}

			// Release the "token" back to the semaphore when done.
			defer func() {
				<-sem
				wg.Done()
			}()

			endpoint := c.Endpoint("/models", modelID)
			resp := &ModelDetails{}
			// err := makeJsonRequest(c, http.MethodGet, endpoint, nil, resp)

			req, err := newJsonRequest(c, http.MethodGet, endpoint, nil)
			if err != nil {
				data[i], errs[i] = resp, err
				return
			}

			_, err = doRequest(c, req, resp)
			data[i], errs[i] = resp, err
		}(i, modelID)
	}

	// Wait for all goroutines to finish.
	wg.Wait()

	// Get the first error from the list.
	for _, err := range errs {
		if err != nil {
			return ModelsData{}, err
		}
	}

	return data, nil
}

// ModelDelete removes a model.
func (c *Client) ModelDelete(model string) (*ModelDeleteResponse, error) {
	endpoint := c.Endpoint("/models", model)
	resp := &ModelDeleteResponse{}

	req, err := newJsonRequest(c, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &ModelDeleteResponse{}, err
	}

	_, err = doRequest(c, req, resp)
	if err != nil {
		return &ModelDeleteResponse{}, err
	}

	return resp, err
}

// Completion returns a list of completions.
func (c *Client) Completion(
	r *CompletionRequest,
) (*CompletionResponse, error) {
	endpoint := c.Endpoint("/completions")
	resp := &CompletionResponse{}

	if err := r.Error(); err != nil {
		return resp, err
	}

	req, err := newJsonRequest(c, http.MethodPost, endpoint, r)
	if err != nil {
		return &CompletionResponse{}, err
	}

	_, err = doRequest(c, req, resp)
	if err != nil {
		return &CompletionResponse{}, err
	}

	return resp, err
}

func (c *Client) ChatCompletion(
	r *ChatCompletionRequest,
) (*ChatCompletionResponse, error) {
	endpoint := c.Endpoint("/chat/completions")
	resp := &ChatCompletionResponse{}

	if err := r.Error(); err != nil {
		return resp, err
	}

	req, err := newJsonRequest(c, http.MethodPost, endpoint, r)
	if err != nil {
		return &ChatCompletionResponse{}, err
	}

	_, err = doRequest(c, req, resp)
	if err != nil {
		return &ChatCompletionResponse{}, err
	}

	return resp, err
}

func (c *Client) Edit(
	r *EditRequest,
) (*EditResponse, error) {
	endpoint := c.Endpoint("/edits")
	resp := &EditResponse{}

	if err := r.Error(); err != nil {
		return resp, err
	}

	req, err := newJsonRequest(c, http.MethodPost, endpoint, r)
	if err != nil {
		return &EditResponse{}, err
	}

	_, err = doRequest(c, req, resp)
	if err != nil {
		return &EditResponse{}, err
	}

	return resp, err
}

func (c *Client) ImageGeneration(
	r *ImageGenerationRequest,
) (*ImageGenerationResponse, error) {
	endpoint := c.Endpoint("/images/generations")
	resp := &ImageGenerationResponse{
		parallelTasks: g.Value(c.parallelTasks, parallelTasks),
	}

	if err := r.Error(); err != nil {
		return resp, err
	}

	req, err := newJsonRequest(c, http.MethodPost, endpoint, r)
	if err != nil {
		return &ImageGenerationResponse{}, err
	}

	_, err = doRequest(c, req, resp)
	if err != nil {
		return &ImageGenerationResponse{}, err
	}

	return resp, err
}

func (c *Client) ImageEdit(
	r *ImageEditRequest,
) (*ImageEditResponse, error) {
	endpoint := c.Endpoint("/images/edits")
	resp := &ImageEditResponse{
		parallelTasks: g.Value(c.parallelTasks, parallelTasks),
	}

	if err := r.Error(); err != nil {
		return resp, err
	}

	req, err := newDataRequest(c, http.MethodPost, endpoint, r)
	if err != nil {
		return &ImageEditResponse{}, err
	}

	_, err = doRequest(c, req, resp)
	if err != nil {
		return &ImageEditResponse{}, err
	}

	return resp, err
}

func (c *Client) ImageVariation(
	r *ImageVariationRequest,
) (*ImageVariationResponse, error) {
	endpoint := c.Endpoint("/images/variations")
	resp := &ImageVariationResponse{
		parallelTasks: g.Value(c.parallelTasks, parallelTasks),
	}

	if err := r.Error(); err != nil {
		return resp, err
	}

	req, err := newDataRequest(c, http.MethodPost, endpoint, r)
	if err != nil {
		return &ImageVariationResponse{}, err
	}

	_, err = doRequest(c, req, resp)
	if err != nil {
		return &ImageVariationResponse{}, err
	}

	return resp, err
}

func (c *Client) Embedding(
	r *EmbeddingRequest,
) (*EmbeddingResponse, error) {
	endpoint := c.Endpoint("/embeddings")
	resp := &EmbeddingResponse{}

	if err := r.Error(); err != nil {
		return resp, err
	}

	req, err := newJsonRequest(c, http.MethodPost, endpoint, r)
	if err != nil {
		return &EmbeddingResponse{}, err
	}

	_, err = doRequest(c, req, resp)
	if err != nil {
		return &EmbeddingResponse{}, err
	}

	return resp, err
}

func (c *Client) AudioTranscription(
	r *AudioTranscriptionRequest,
) (*AudioTranscriptionResponse, error) {
	endpoint := c.Endpoint("/audio/transcriptions")
	resp := &AudioTranscriptionResponse{}

	if err := r.Error(); err != nil {
		return resp, err
	}

	req, err := newDataRequest(c, http.MethodPost, endpoint, r)
	if err != nil {
		return &AudioTranscriptionResponse{}, err
	}

	_, err = doRequest(c, req, resp)
	if err != nil {
		return &AudioTranscriptionResponse{}, err
	}

	return resp, err
}

func (c *Client) AudioTranslation(
	r *AudioTranslationRequest,
) (*AudioTranslationResponse, error) {
	endpoint := c.Endpoint("/audio/translations")
	resp := &AudioTranslationResponse{}

	if err := r.Error(); err != nil {
		return resp, err
	}

	req, err := newDataRequest(c, http.MethodPost, endpoint, r)
	if err != nil {
		return &AudioTranslationResponse{}, err
	}

	_, err = doRequest(c, req, resp)
	if err != nil {
		return &AudioTranslationResponse{}, err
	}

	return resp, err
}

func (c *Client) Files(files ...string) (FilesData, error) {
	var wg sync.WaitGroup

	// If no files are provided, get all files.
	if len(files) == 0 {
		endpoint := c.Endpoint("/files")
		resp := &FileResponse{}

		req, err := newJsonRequest(c, http.MethodGet, endpoint, nil)
		if err != nil {
			return FilesData{}, err
		}

		_, err = doRequest(c, req, resp)
		if err != nil {
			return FilesData{}, err
		}

		return resp.Data, nil
	}

	data := make(FilesData, len(files))
	errs := make([]error, len(files))

	// Create a buffered channel with a capacity equal
	// to the number of CPU cores.
	sem := make(chan struct{}, c.ParallelTasks())

	for i, modelID := range files {
		wg.Add(1)
		go func(i int, modelID string) {
			// Acquire a "token" from the semaphore.
			sem <- struct{}{}

			// Release the "token" back to the semaphore when done.
			defer func() {
				<-sem
				wg.Done()
			}()

			endpoint := c.Endpoint("/models", modelID)
			resp := &FileDetails{}
			// err := makeJsonRequest(c, http.MethodGet, endpoint, nil, resp)

			req, err := newJsonRequest(c, http.MethodGet, endpoint, nil)
			if err != nil {
				data[i], errs[i] = resp, err
				return
			}

			_, err = doRequest(c, req, resp)
			data[i], errs[i] = resp, err
		}(i, modelID)
	}

	// Wait for all goroutines to finish.
	wg.Wait()

	// Get the first error from the list.
	for _, err := range errs {
		if err != nil {
			return FilesData{}, err
		}
	}

	return data, nil
}

// FileDelete removes a file.
func (c *Client) FileDelete(file string) (*FileDeleteResponse, error) {
	endpoint := c.Endpoint("/files", file)
	resp := &FileDeleteResponse{}

	req, err := newJsonRequest(c, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &FileDeleteResponse{}, err
	}

	_, err = doRequest(c, req, resp)
	if err != nil {
		return &FileDeleteResponse{}, err
	}

	return resp, err
}

func (c *Client) FileUpload(
	r *FileUploadRequest,
) (*FileUploadResponse, error) {
	endpoint := c.Endpoint("/files")
	resp := &FileUploadResponse{}

	if err := r.Error(); err != nil {
		return resp, err
	}

	req, err := newDataRequest(c, http.MethodPost, endpoint, r)
	if err != nil {
		return &FileUploadResponse{}, err
	}

	_, err = doRequest(c, req, resp)
	if err != nil {
		return &FileUploadResponse{}, err
	}

	return resp, err
}

func (c *Client) FileContent(file string) (string, error) {
	endpoint := c.Endpoint("/files", file, "content")
	req, err := newJsonRequest(c, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}

	resp, err := doRequest(c, req, nil)
	if err != nil {
		return "", err
	}

	return string(resp), nil
}

func (c *Client) FineTune(
	r *FineTuneRequest,
) (*FineTuneResponse, error) {
	endpoint := c.Endpoint("/fine-tunes")
	resp := &FineTuneResponse{}

	req, err := newJsonRequest(c, http.MethodPost, endpoint, r)
	if err != nil {
		return &FineTuneResponse{}, err
	}

	_, err = doRequest(c, req, resp)
	if err != nil {
		return &FineTuneResponse{}, err
	}

	return resp, err
}

func (c *Client) FineTunes(fineTunes ...string) (FineTunesData, error) {
	var wg sync.WaitGroup

	// If no modelIDs are provided, get all models.
	if len(fineTunes) == 0 {
		endpoint := c.Endpoint("/fine-tunes")
		resp := &FineTuneListResponse{}

		req, err := newJsonRequest(c, http.MethodGet, endpoint, nil)
		if err != nil {
			return FineTunesData{}, err
		}

		_, err = doRequest(c, req, resp)
		if err != nil {
			return FineTunesData{}, err
		}

		return resp.Data, nil
	}

	data := make(FineTunesData, len(fineTunes))
	errs := make([]error, len(fineTunes))

	// Create a buffered channel with a capacity equal
	// to the number of CPU cores.
	sem := make(chan struct{}, c.ParallelTasks())

	for i, fineTuneID := range fineTunes {
		wg.Add(1)
		go func(i int, fineTuneID string) {
			// Acquire a "token" from the semaphore.
			sem <- struct{}{}

			// Release the "token" back to the semaphore when done.
			defer func() {
				<-sem
				wg.Done()
			}()

			endpoint := c.Endpoint("/fine-tunes", fineTuneID)
			resp := &FineTuneResponse{}
			// err := makeJsonRequest(c, http.MethodGet, endpoint, nil, resp)

			req, err := newJsonRequest(c, http.MethodGet, endpoint, nil)
			if err != nil {
				data[i], errs[i] = resp, err
				return
			}

			_, err = doRequest(c, req, resp)
			data[i], errs[i] = resp, err
		}(i, fineTuneID)
	}

	// Wait for all goroutines to finish.
	wg.Wait()

	// Get the first error from the list.
	for _, err := range errs {
		if err != nil {
			return FineTunesData{}, err
		}
	}

	return data, nil
}

func (c *Client) FineTuneCancel(fineTune string) (*FineTuneResponse, error) {
	endpoint := c.Endpoint("/fine-tunes", fineTune, "cancel")
	resp := &FineTuneResponse{}

	req, err := newJsonRequest(c, http.MethodPost, endpoint, nil)
	if err != nil {
		return &FineTuneResponse{}, err
	}

	_, err = doRequest(c, req, resp)
	if err != nil {
		return &FineTuneResponse{}, err
	}

	return resp, err
}

func (c *Client) FineTuneEvents(fineTune string) (FineTuneEventsData, error) {
	endpoint := c.Endpoint("/fine-tunes", fineTune, "events")
	resp := &FineTuneEventListResponse{}

	req, err := newJsonRequest(c, http.MethodGet, endpoint, nil)
	if err != nil {
		return FineTuneEventsData{}, err
	}

	_, err = doRequest(c, req, resp)
	if err != nil {
		return FineTuneEventsData{}, err
	}

	return resp.Data, nil
}

func (c *Client) Moderation(
	r *ModerationRequest,
) (*ModerationResponse, error) {
	endpoint := c.Endpoint("/moderations")
	resp := &ModerationResponse{}

	if err := r.Error(); err != nil {
		return resp, err
	}

	req, err := newJsonRequest(c, http.MethodPost, endpoint, r)
	if err != nil {
		return &ModerationResponse{}, err
	}

	_, err = doRequest(c, req, resp)
	if err != nil {
		return &ModerationResponse{}, err
	}

	return resp, err
}
