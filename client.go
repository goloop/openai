package openai

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/goloop/g"
)

// The statement var _ Clienter = (*Client)(nil) is a compile-time check to
// ensure that *Client implements the Clienter interface. If *Client does not
// implement every function in the Clienter interface, the program will fail
// to compile. It's a good practice in Go to check whether certain type
// implements specific interface.
var _ Clienter = (*Client)(nil)

// Clienter interface defines methods for interacting with OpenAI API client.
// It abstracts the implementation details and exposes methods that can be
// used regardless of the specific implementation of the client.
type Clienter interface {
	APIKey() string
	OrgID() string
	Endpoint(...string) string
	ParallelTasks() int
	Context() context.Context
	HTTPHeaders() http.Header
	HTTPClient() *http.Client
}

// Requester interface defines methods to manage requests to the OpenAI API.
// It provides a uniform way of handling errors and flushing request data,
// regardless of the specific implementation of the requests.
type Requester interface {
	Error() error
	Flush()
}

// Config represents the OpenAI API client's configuration parameters.
// It includes necessary details for creating a client such as API key,
// organization ID, and base URL. It also includes parameters for managing
// requests like parallel task count, request timeout, and HTTP headers.
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

// Client represents the OpenAI API client. It includes fields that hold
// configuration parameters and implements methods defined in Clienter
// interface. This structure allows interacting with OpenAI API by sending
// requests and processing responses.
//
// Although Client has the same fields as Config, it does not embed Config
// directly to avoid exposing Config's fields directly. This provides more
// control over how and when the client's state can be changed.
type Client struct {
	apiKey     string // secret key for authorization
	orgID      string // unique identifier of the organization
	apiBaseURL string // base URL of OpenAI API

	parallelTasks int             // number of parallel requests
	context       context.Context // context for requests
	httpHeaders   http.Header     // additional HTTP headers for requests
	httpClient    *http.Client    // http client for sending requests
}

// Error checks the current configuration of the OpenAI API client and
// returns an error if something crucial is missing or incorrect.
// It validates that all the required parameters (APIKey, APIBaseURL,
// HTTPClient, and Context) are provided and correct.
func (c *Client) Error() error {
	// APIKey is a required parameter. Without it, we cannot authenticate
	// with the OpenAI API. If it's missing, we return an ErrNoAPIKey error.
	if c.apiKey == "" {
		return ErrNoAPIKey
	}

	// APIBaseURL is a required parameter. It's the base URL to which
	// we append endpoint paths when making API requests. If it's missing,
	// we return an ErrNoAPIBaseURL error. We also check if the URL is
	// well-formed.
	if c.apiBaseURL == "" {
		return ErrNoAPIBaseURL
	} else if _, err := urlBuild(c.apiBaseURL); err != nil {
		return err // return an error if the URL is not well-formed
	}

	// HTTPClient is a required parameter. It's used to send HTTP requests
	// to the OpenAI API. If it's missing, we return an ErrNoHTTPClient error.
	if c.httpClient == nil {
		return ErrNoHTTPClient
	}

	// Context is a required parameter. It's used to control cancellation
	// and timeouts for API requests. If it's missing, we return an
	// ErrNoContext error.
	if c.context == nil {
		return ErrNoContext
	}

	// If all the required parameters are correct, we return nil indicating
	// that the client configuration is okay.
	return nil
}

// Configure updates the configuration of the client using the provided
// Config object. If a configuration field in the Config object is not
// set, the existing value in the Client object is preserved. If some
// fields in the Client object are not set either, they are initialized
// with default values.
func (c *Client) Configure(config Config) {
	// APIKey is updated if a new one is provided,
	// else the existing one is kept.
	c.apiKey = g.Value(config.APIKey, c.apiKey)

	// OrgID is updated if a new one is provided,
	// else the existing one is kept.
	c.orgID = g.Value(config.OrgID, c.orgID)

	// APIBaseURL is updated if a new one is provided,
	// else the existing one is kept. If both are not set,
	// the default apiBaseURL is used.
	c.apiBaseURL = g.Value(config.APIBaseURL, c.apiBaseURL, apiBaseURL)

	// The number of parallel tasks is updated if a new value is provided,
	// else the existing one is kept. If both are not set, the default
	// parallelTasks value is used.
	c.parallelTasks = g.Value(
		config.ParallelTasks,
		c.parallelTasks,
		parallelTasks,
	)

	// Context is updated if a new one is provided, else the
	// existing one is kept. If both are not set, the background
	// context is used.
	c.context = g.Value(config.Context, c.context, context.Background())

	// HTTPHeaders are updated if new ones are provided,
	// else the existing ones are kept.
	c.httpHeaders = g.Value(config.HTTPHeaders, c.httpHeaders)

	// HTTPClient is updated if a new one is provided,
	// else the existing one is kept. If both are not set,
	// a new default HTTP client with a set timeout is used.
	c.httpClient = g.Value(
		config.HTTPClient,
		c.httpClient,
		&http.Client{
			Timeout: requestTimeout,
		},
	)
}

// APIKey returns the API key used for authentication with the OpenAI API.
func (c *Client) APIKey() string {
	return c.apiKey
}

// OrgID returns the unique identifier of the organization.
func (c *Client) OrgID() string {
	return c.orgID
}

// Endpoint concatenates the base API URL with the provided path elements
// to create a complete endpoint URL. If the URL building process fails,
// it simply returns the base API URL.
func (c *Client) Endpoint(p ...string) string {
	u, _ := urlBuild(c.apiBaseURL, p...)
	return u
}

// ParallelTasks returns the number of requests that can be made in parallel.
func (c *Client) ParallelTasks() int {
	return c.parallelTasks
}

// Context returns the context.Context that should be used
// for HTTP requests. The context controls cancellation of
// requests and carries request-scoped data.
func (c *Client) Context() context.Context {
	return c.context
}

// HTTPHeaders returns the additional HTTP headers that should be
// included in requests.
func (c *Client) HTTPHeaders() http.Header {
	return c.httpHeaders
}

// HTTPClient returns the http.Client that should be used
// to send HTTP requests.
func (c *Client) HTTPClient() *http.Client {
	return c.httpClient
}

// Models returns the client for https://api.openai.com/v1/models
// The function performs parallel HTTP GET requests to fetch details
// about one or multiple models. If no modelIDs are provided, it
// fetches data about all available models.
func (c *Client) Models(models ...string) (ModelsData, error) {
	var wg sync.WaitGroup

	// If no modelIDs are provided, a GET request is made to the /models
	// endpoint to fetch data about all available models.
	if len(models) == 0 {
		endpoint := c.Endpoint("/models")
		resp := &ModelResponse{}

		req, err := newJSONRequest(c, http.MethodGet, endpoint, nil)
		if err != nil {
			return ModelsData{}, err
		}

		_, err = doRequest(c, req, resp)
		if err != nil {
			return ModelsData{}, err
		}

		return resp.Data, nil
	}

	// Prepare the structures to hold the data and any possible errors.
	data := make(ModelsData, len(models))
	errs := make([]error, len(models))

	// Create a buffered channel (a semaphore) with a capacity equal
	// to the number of CPU cores to control the number of
	// concurrent goroutines.
	sem := make(chan struct{}, c.ParallelTasks())

	for i, modelID := range models {
		wg.Add(1)
		go func(i int, modelID string) {
			// Acquire a "token" from the semaphore.
			sem <- struct{}{}

			// Ensure to release the "token" back to the semaphore and
			// mark the goroutine as done when finished.
			defer func() {
				<-sem
				wg.Done()
			}()

			endpoint := c.Endpoint("/models", modelID)
			resp := &ModelDetails{}

			req, err := newJSONRequest(c, http.MethodGet, endpoint, nil)
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

	// If any of the requests returned an error,
	// return the first encountered error.
	for _, err := range errs {
		if err != nil {
			return ModelsData{}, err
		}
	}

	// If no errors occurred, return the fetched model data.
	return data, nil
}

// ModelDelete removes a fine-tuned model from the OpenAI API.
// The endpoint for this function is "https://api.openai.com/v1/models/{model}".
// To successfully delete a model, the client must have the "Owner"
// role within their organization. If the deletion is successful, it returns
// a response without an error. Otherwise, an error is returned along with
// an empty response.
func (c *Client) ModelDelete(model string) (*ModelDeleteResponse, error) {
	endpoint := c.Endpoint("/models", model) // endpoint to delete the model
	resp := &ModelDeleteResponse{}           // response data

	req, err := newJSONRequest(c, http.MethodDelete, endpoint, nil)
	// Error is returned if there was an issue creating the request.
	if err != nil {
		return &ModelDeleteResponse{}, err
	}

	_, err = doRequest(c, req, resp)
	if err != nil {
		return &ModelDeleteResponse{}, err
	}

	// If no errors occur, return the response and nil for the error.
	return resp, err
}

// Completion generates a list of predicted completions for the given prompt.
// The endpoint for this function is "https://api.openai.com/v1/completions".
// Given a prompt, the model will return one or more predicted completions.
// The API can also return the probabilities of alternative tokens at each
// position. This method takes a CompletionRequest as input, and returns a
// CompletionResponse. If there's an error with the CompletionRequest, the
// error is returned immediately. If there's an error creating or sending
// the HTTP request, it returns an error along with an empty response.
func (c *Client) Completion(
	r *CompletionRequest,
) (*CompletionResponse, error) {
	// Defines the API endpoint to call for generating completions.
	endpoint := c.Endpoint("/completions")

	// Container for the response data.
	resp := &CompletionResponse{}

	// If there is an error with the provided CompletionRequest,
	// return the error.
	if err := r.Error(); err != nil {
		return resp, err
	}

	// Create a new JSON request to send to the API.
	req, err := newJSONRequest(c, http.MethodPost, endpoint, r)

	// Error is returned if there was an issue creating the request.
	if err != nil {
		return &CompletionResponse{}, err
	}

	// Execute the HTTP request and populate the response container.
	_, err = doRequest(c, req, resp)

	// Error is returned if there was an issue executing the request.
	if err != nil {
		return &CompletionResponse{}, err
	}

	// If no errors occur, return the populated
	// response and nil for the error.
	return resp, err
}

// ChatCompletion generates a model response for the given chat conversation.
// The endpoint for this function is "https://api.openai.com/v1/chat/completions".
// The method takes a ChatCompletionRequest as input and returns a
// ChatCompletionResponse. If there's an error with the ChatCompletionRequest,
// the error is returned immediately. If there's an error creating or sending
// the HTTP request, it returns an error along with an empty response.
//
// The ChatCompletionRequest should include the following fields:
//   - model: ID of the model to use. See the model endpoint compatibility
//     table for details on which models work with the Chat API;
//   - messages: A list of messages comprising the conversation so far.
func (c *Client) ChatCompletion(
	r *ChatCompletionRequest,
) (*ChatCompletionResponse, error) {
	// Defines the API endpoint to call for generating chat completions.
	endpoint := c.Endpoint("/chat/completions")

	// Container for the response data
	resp := &ChatCompletionResponse{}

	// If there is an error with the provided ChatCompletionRequest,
	// return the error.
	if err := r.Error(); err != nil {
		return resp, err
	}

	// Create a new JSON request to send to the API.
	req, err := newJSONRequest(c, http.MethodPost, endpoint, r)

	// Error is returned if there was an issue creating the request.
	if err != nil {
		return &ChatCompletionResponse{}, err
	}

	// Execute the HTTP request and populate the response container.
	_, err = doRequest(c, req, resp)

	// Error is returned if there was an issue executing the request.
	if err != nil {
		return &ChatCompletionResponse{}, err
	}

	// If no errors occur, return the populated response and nil for the error.
	return resp, err
}

// Edit generates an edited version of the provided prompt based on
// the provided instruction.
// The endpoint for this function is "https://api.openai.com/v1/edits".
// This method takes an EditRequest as input and returns an EditResponse.
// The EditRequest should contain the input, instruction, and necessary
// parameters for the editing task. If there's an error with the EditRequest,
// the error is returned immediately. If there's an error creating or sending
// the HTTP request, it returns an error along with an empty response.
func (c *Client) Edit(
	r *EditRequest,
) (*EditResponse, error) {
	// Defines the API endpoint to call for generating edits.
	endpoint := c.Endpoint("/edits")

	// Container for the response data
	resp := &EditResponse{}

	// If there is an error with the provided EditRequest, return the error.
	if err := r.Error(); err != nil {
		return resp, err
	}

	// Create a new JSON request to send to the API.
	req, err := newJSONRequest(c, http.MethodPost, endpoint, r)

	// Error is returned if there was an issue creating the request.
	if err != nil {
		return &EditResponse{}, err
	}

	// Execute the HTTP request and populate the response container.
	_, err = doRequest(c, req, resp)

	// Error is returned if there was an issue executing the request.
	if err != nil {
		return &EditResponse{}, err
	}

	// If no errors occur, return the populated response and nil for the error.
	return resp, err
}

// ImageGeneration generates an image based on the provided text description.
// The endpoint for this function is "https://api.openai.com/v1/images/generations".
// This method takes an ImageGenerationRequest as input and returns an
// ImageGenerationResponse. The ImageGenerationRequest should contain a text
// prompt, number of images to generate, size of the generated images, response
// format, and an optional user identifier. If there's an error with the
// ImageGenerationRequest, the error is returned immediately.
// If there's an error creating or sending the HTTP request, it returns an
// error along with an empty response.
func (c *Client) ImageGeneration(
	r *ImageGenerationRequest,
) (*ImageGenerationResponse, error) {
	// Defines the API endpoint to call for image generation.
	endpoint := c.Endpoint("/images/generations")

	// Container for the response data
	resp := &ImageGenerationResponse{
		parallelTasks: g.Value(c.parallelTasks, parallelTasks),
	}

	// If there is an error with the provided ImageGenerationRequest,
	// return the error.
	if err := r.Error(); err != nil {
		return resp, err
	}

	// Create a new JSON request to send to the API.
	req, err := newJSONRequest(c, http.MethodPost, endpoint, r)
	if err != nil {
		return &ImageGenerationResponse{}, err
	}

	// Execute the HTTP request and populate the response container.
	_, err = doRequest(c, req, resp)
	if err != nil {
		return &ImageGenerationResponse{}, err
	}

	// If no errors occur, return the populated
	// response and nil for the error.
	return resp, err
}

// ImageEdit creates an edited or extended image based on the provided
// original image and a text prompt.
// The endpoint for this function is "https://api.openai.com/v1/images/edits".
// This method takes an ImageEditRequest as input and returns an
// ImageEditResponse. The ImageEditRequest should contain an original image
// and a text prompt for editing. If there's an error with the
// ImageEditRequest, the error is returned immediately.
// If there's an error creating or sending the HTTP request,
// it returns an error along with an empty response.
func (c *Client) ImageEdit(
	r *ImageEditRequest,
) (*ImageEditResponse, error) {
	// Defines the API endpoint to call for image editing
	endpoint := c.Endpoint("/images/edits")

	// Container for the response data
	resp := &ImageEditResponse{
		parallelTasks: g.Value(c.parallelTasks, parallelTasks),
	}

	// If there is an error with the provided ImageEditRequest,
	// return the error.
	if err := r.Error(); err != nil {
		return resp, err
	}

	// Create a new data request to send to the API.
	req, err := newDataRequest(c, http.MethodPost, endpoint, r)
	if err != nil {
		return &ImageEditResponse{}, err
	}

	// Execute the HTTP request and populate the response container.
	_, err = doRequest(c, req, resp)
	if err != nil {
		return &ImageEditResponse{}, err
	}

	// If no errors occur, return the populated response
	// and nil for the error.
	return resp, err
}

// ImageVariation generates a variation of a given image based on the
// provided input parameters.
// The endpoint for this function is "https://api.openai.com/v1/images/variations".
// This method takes an ImageVariationRequest as input and returns an
// ImageVariationResponse. The ImageVariationRequest should contain a base
// image to use for the variation(s). If there's an error with the
// ImageVariationRequest, the error is returned immediately.
// If there's an error creating or sending the HTTP request,
// it returns an error along with an empty response.
func (c *Client) ImageVariation(
	r *ImageVariationRequest,
) (*ImageVariationResponse, error) {
	// Defines the API endpoint to call for image variation.
	endpoint := c.Endpoint("/images/variations")

	// Container for the response data.
	resp := &ImageVariationResponse{
		parallelTasks: g.Value(c.parallelTasks, parallelTasks),
	}

	// If there is an error with the provided ImageVariationRequest,
	// return the error.
	if err := r.Error(); err != nil {
		return resp, err
	}

	// Create a new data request to send to the API.
	req, err := newDataRequest(c, http.MethodPost, endpoint, r)
	if err != nil {
		return &ImageVariationResponse{}, err
	}

	// Execute the HTTP request and populate the response container.
	_, err = doRequest(c, req, resp)
	if err != nil {
		return &ImageVariationResponse{}, err
	}

	// If no errors occur, return the populated response
	// and nil for the error.
	return resp, err
}

// Embedding function returns a vector representation of a given input.
// This vector can be easily consumed by machine learning models and algorithms.
// The endpoint for this function is "https://api.openai.com/v1/embeddings".
// This function takes an EmbeddingRequest as input and returns an
// EmbeddingResponse. The EmbeddingRequest should contain the text for
// which an embedding representation is required. If there's an error with the
// EmbeddingRequest, the error is returned immediately.
// If there's an error creating or sending the HTTP request,
// it returns an error along with an empty response.
func (c *Client) Embedding(
	r *EmbeddingRequest,
) (*EmbeddingResponse, error) {
	// Defines the API endpoint to call for creating embeddings.
	endpoint := c.Endpoint("/embeddings")

	// Container for the response data.
	resp := &EmbeddingResponse{}
	if err := r.Error(); err != nil {
		return resp, err
	}

	// Create a new JSON request to send to the API.
	req, err := newJSONRequest(c, http.MethodPost, endpoint, r)
	if err != nil {
		return &EmbeddingResponse{}, err
	}

	// Execute the HTTP request and populate the response container.
	_, err = doRequest(c, req, resp)
	if err != nil {
		return &EmbeddingResponse{}, err
	}

	// If no errors occur, return the populated response
	// and nil for the error.
	return resp, err
}

// AudioTranscription function transcribes audio into text. The endpoint
// for this function is "https://api.openai.com/v1/audio/transcriptions".
// This function takes an AudioTranscriptionRequest as input and returns
// an AudioTranscriptionResponse. The AudioTranscriptionRequest should
// contain the audio data to be transcribed. If there's an error with the
// AudioTranscriptionRequest, the error is returned immediately.
// If there's an error creating or sending the HTTP request,
// it returns an error along with an empty response.
func (c *Client) AudioTranscription(
	r *AudioTranscriptionRequest,
) (*AudioTranscriptionResponse, error) {
	// Defines the API endpoint to call for creating audio transcriptions.
	endpoint := c.Endpoint("/audio/transcriptions")

	// Container for the response data.
	resp := &AudioTranscriptionResponse{}
	if err := r.Error(); err != nil {
		return resp, err
	}

	// Create a new data request to send to the API.
	req, err := newDataRequest(c, http.MethodPost, endpoint, r)
	if err != nil {
		return &AudioTranscriptionResponse{}, err
	}

	// Execute the HTTP request and populate the response container.
	_, err = doRequest(c, req, resp)
	if err != nil {
		return &AudioTranscriptionResponse{}, err
	}

	// If no errors occur, return the populated response
	// and nil for the error.
	return resp, err
}

// AudioTranslation function translates audio into English. The endpoint for
// this function is "https://api.openai.com/v1/audio/translations".
// This function takes an AudioTranslationRequest as input and returns an
// AudioTranslationResponse. The AudioTranslationRequest should contain the
// audio data to be translated. If there's an error with the
// AudioTranslationRequest, the error is returned immediately.
// If there's an error creating or sending the HTTP request,
// it returns an error along with an empty response.
func (c *Client) AudioTranslation(
	r *AudioTranslationRequest,
) (*AudioTranslationResponse, error) {
	// Defines the API endpoint to call for translating audio to English.
	endpoint := c.Endpoint("/audio/translations")

	// Container for the response data.
	resp := &AudioTranslationResponse{}
	if err := r.Error(); err != nil {
		return resp, err
	}

	// Create a new data request to send to the API.
	req, err := newDataRequest(c, http.MethodPost, endpoint, r)
	if err != nil {
		return &AudioTranslationResponse{}, err
	}

	// Execute the HTTP request and populate the response container.
	_, err = doRequest(c, req, resp)
	if err != nil {
		return &AudioTranslationResponse{}, err
	}

	// If no errors occur, return the populated response
	// and nil for the error.
	return resp, err
}

// Files function fetches details of all the files or a specific set of
// files based on the provided parameters.
// The endpoint for this function is "https://api.openai.com/v1/files".
// This function takes a variable number of file names as strings.
// If no file names are provided, it fetches details of all the files.
// It returns a FilesData type that contains details of all the requested
// files, and an error if there was an issue in the request.
// It leverages go routines and channels to parallelize the requests
// for each file, improving the function's performance.
func (c *Client) Files(files ...string) (FilesData, error) {
	var wg sync.WaitGroup

	// If no files are provided, get all files.
	if len(files) == 0 {
		endpoint := c.Endpoint("/files")
		resp := &FileResponse{}

		req, err := newJSONRequest(c, http.MethodGet, endpoint, nil)
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

	// For each provided file, create a new goroutine
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

			req, err := newJSONRequest(c, http.MethodGet, endpoint, nil)
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

	// Return the gathered data and nil as no errors occurred.
	return data, nil
}

// FileDelete is a function that deletes a specific file from the user's
// files on the OpenAI server.
// It requires the ID of the file to be deleted as an input string parameter.
// The endpoint for this function is "https://api.openai.com/v1/files/{file_id}"
// where {file_id} is the ID of the file to be deleted.
// If the operation is successful, it returns a FileDeleteResponse and nil error.
// If there's an error with the operation, it will return an empty
// FileDeleteResponse and an error detailing the issue.
func (c *Client) FileDelete(file string) (*FileDeleteResponse, error) {
	// Construct the endpoint with the provided file id.
	endpoint := c.Endpoint("/files", file)
	resp := &FileDeleteResponse{}

	// Create a new DELETE request.
	req, err := newJSONRequest(c, http.MethodDelete, endpoint, nil)
	if err != nil {
		// If there's an error while creating the request,
		//return an empty response and the error.
		return &FileDeleteResponse{}, err
	}

	// Perform the request.
	_, err = doRequest(c, req, resp)
	if err != nil {
		// If there's an error while performing the request,
		// return an empty response and the error.
		return &FileDeleteResponse{}, err
	}

	// If there are no errors, return the response and nil error.
	return resp, err
}

// FileUpload is a function that uploads a file to the OpenAI server.
// The file contains document(s) that can be used across various OpenAI
// endpoints/features.
// The function requires a FileUploadRequest as an argument, which should
// include the name of the file and the intended purpose of the uploaded
// documents.
//
// If the purpose is "fine-tune", each line of the uploaded JSON Lines
// file should be a JSON record with "prompt" and "completion" fields
// representing your training examples.
//
// The endpoint for this function is "https://api.openai.com/v1/files".
// If the operation is successful, it returns a FileUploadResponse and
// nil error.
//
// If there's an error with the operation, it will return an empty
// FileUploadResponse and an error detailing the issue.
func (c *Client) FileUpload(
	r *FileUploadRequest,
) (*FileUploadResponse, error) {
	// Construct the endpoint.
	endpoint := c.Endpoint("/files")
	resp := &FileUploadResponse{}

	// Check for errors in the request.
	if err := r.Error(); err != nil {
		return resp, err
	}

	// Create a new POST request.
	req, err := newDataRequest(c, http.MethodPost, endpoint, r)
	if err != nil {
		// If there's an error while creating the request,
		// return an empty response and the error.
		return &FileUploadResponse{}, err
	}

	// Perform the request.
	_, err = doRequest(c, req, resp)
	if err != nil {
		// If there's an error while performing the request,
		// return an empty response and the error.
		return &FileUploadResponse{}, err
	}

	// If there are no errors, return the response and nil error.
	return resp, err
}

// FileContent is a function that retrieves the contents of
// a specific file from the OpenAI server.
// The function requires the file's ID as a string as an argument.
// The endpoint for this function is
// "https://api.openai.com/v1/files/{file_id}/content", where {file_id}
// is replaced with the ID of the file you want to retrieve.
//
// If the operation is successful, it returns the contents of the file as
// a string and a nil error. If there's an error with the operation, it will
// return an empty string and an error detailing the issue.
func (c *Client) FileContent(file string) (string, error) {
	// Construct the endpoint using the provided file ID.
	endpoint := c.Endpoint("/files", file, "content")

	// Create a new GET request.
	req, err := newJSONRequest(c, http.MethodGet, endpoint, nil)
	if err != nil {
		// If there's an error while creating the request,
		// return an empty string and the error.
		return "", err
	}

	// Perform the request.
	resp, err := doRequest(c, req, nil)
	if err != nil {
		// If there's an error while performing the request,
		// return an empty string and the error.
		return "", err
	}

	// If there are no errors, return the content of the file and nil error.
	return string(resp), nil
}

// FineTune is a function that initiates a fine-tuning process on a model.
// The purpose of fine-tuning is to adapt a pre-trained model to a specific
// task using a custom dataset.
// This function takes a FineTuneRequest struct as a parameter, which includes
// information about the model and dataset to be used.
// The endpoint for this function is "https://api.openai.com/v1/fine-tunes".
// If the operation is successful, it returns a FineTuneResponse struct,
// which includes details about the initiated fine-tuning job.
// The response also includes the status of the job and the name of the
// fine-tuned model upon completion. If there's an error with the operation,
// it will return a FineTuneResponse struct initialized with default values
// and an error detailing the issue.
func (c *Client) FineTune(
	r *FineTuneRequest,
) (*FineTuneResponse, error) {
	// Construct the endpoint URL for the fine-tuning process.
	endpoint := c.Endpoint("/fine-tunes")

	// Prepare the response struct.
	resp := &FineTuneResponse{}

	// Create a new POST request.
	req, err := newJSONRequest(c, http.MethodPost, endpoint, r)
	if err != nil {
		// If there's an error while creating the request,
		// return a FineTuneResponse struct initialized with
		// default values and the error.
		return &FineTuneResponse{}, err
	}

	// Perform the request.
	_, err = doRequest(c, req, resp)
	if err != nil {
		// If there's an error while performing the request,
		// return a FineTuneResponse struct initialized with
		// default values and the error.
		return &FineTuneResponse{}, err
	}

	// If the operation is successful,
	// return the response and a nil error.
	return resp, err
}

// FineTunes is a function that retrieves information about fine-tuning jobs.
// If no fineTuneIDs are provided, it returns a list of all fine-tuning jobs.
// If fineTuneIDs are provided, it retrieves information about the
// specific fine-tuning jobs.
// This function takes a variable number of fineTuneIDs as input.
// The endpoint for retrieving fine-tuning jobs is "https://api.openai.com/v1/fine-tunes".
// If the operation is successful, it returns a FineTunesData struct,
// which contains information about the fine-tuning jobs.
// The response includes details such as the fine-tuning job ID, model ID,
// status, and other relevant information.
// If there's an error with the operation, it will return a FineTunesData
// struct initialized with default values and an error detailing the issue.
func (c *Client) FineTunes(fineTunes ...string) (FineTunesData, error) {
	var wg sync.WaitGroup

	// If no modelIDs are provided, get all models.
	if len(fineTunes) == 0 {
		endpoint := c.Endpoint("/fine-tunes")
		resp := &FineTuneListResponse{}

		req, err := newJSONRequest(c, http.MethodGet, endpoint, nil)
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

			req, err := newJSONRequest(c, http.MethodGet, endpoint, nil)
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

// FineTuneCancel is a function that cancels a specific fine-tuning job.
// It takes the fineTuneID as input, which represents the ID of the
// fine-tuning job to be canceled.
// The endpoint for canceling a fine-tuning job is
// "https://api.openai.com/v1/fine-tunes/{fine_tune_id}/cancel".
// If the operation is successful, it returns a FineTuneResponse,
// which contains information about the canceled fine-tuning job.
// If there's an error with the operation, it will return a FineTuneResponse
// initialized with default values and an error detailing the issue.
func (c *Client) FineTuneCancel(fineTune string) (*FineTuneResponse, error) {
	endpoint := c.Endpoint("/fine-tunes", fineTune, "cancel")
	resp := &FineTuneResponse{}

	req, err := newJSONRequest(c, http.MethodPost, endpoint, nil)
	if err != nil {
		return &FineTuneResponse{}, err
	}

	_, err = doRequest(c, req, resp)
	if err != nil {
		return &FineTuneResponse{}, err
	}

	return resp, err
}

// FineTuneEvents is a function that retrieves fine-grained status updates
// for a specific fine-tuning job. It takes the fineTuneID as input, which
// represents the ID of the fine-tuning job to get events for.
// The endpoint for retrieving fine-tune events is
// "https://api.openai.com/v1/fine-tunes/{fine_tune_id}/events".
// If the operation is successful, it returns a FineTuneEventsData, which
// contains the fine-tune events data.
// If there's an error with the operation, it will return a FineTuneEventsData
// initialized with default values and an error detailing the issue.
func (c *Client) FineTuneEvents(fineTune string) (FineTuneEventsData, error) {
	endpoint := c.Endpoint("/fine-tunes", fineTune, "events")
	resp := &FineTuneEventListResponse{}

	req, err := newJSONRequest(c, http.MethodGet, endpoint, nil)
	if err != nil {
		return FineTuneEventsData{}, err
	}

	_, err = doRequest(c, req, resp)
	if err != nil {
		return FineTuneEventsData{}, err
	}

	return resp.Data, nil
}

// Moderation is a function that checks if the provided input text
// violates OpenAI's content policy.
// It takes a ModerationRequest object as input, which contains the
// text to be checked.
// The endpoint for creating a moderation is "https://api.openai.com/v1/moderations".
// If the operation is successful, it returns a ModerationResponse,
// which contains the moderation result.
// If there's an error with the operation, it will return a ModerationResponse
// initialized with default values and an error detailing the issue.
func (c *Client) Moderation(
	r *ModerationRequest,
) (*ModerationResponse, error) {
	// Construct the endpoint URL for creating a moderation.
	endpoint := c.Endpoint("/moderations")
	resp := &ModerationResponse{}

	// Check for any errors in the request.
	if err := r.Error(); err != nil {
		return resp, err
	}

	// Create a new POST request.
	req, err := newJSONRequest(c, http.MethodPost, endpoint, r)
	if err != nil {
		return &ModerationResponse{}, err
	}

	// Perform the request.
	_, err = doRequest(c, req, resp)
	if err != nil {
		return &ModerationResponse{}, err
	}

	// If the operation is successful,
	// return the response data and a nil error.
	return resp, err
}
