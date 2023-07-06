package openai

import (
	"runtime"
	"time"
)

// parallelTasks sets the default number of parallel requests to twice the
// number of CPUs. It leverages Go's capabilities of utilizing multiple
// cores for concurrent operations.
var parallelTasks = runtime.NumCPU() * 2

const (
	// apiBaseURL defines the default base URL for the OpenAI API.
	// It is used as the base for constructing the endpoints for different API
	// requests.
	apiBaseURL = "https://api.openai.com/v1"

	// requestTimeout sets the default maximum duration time for a request to
	// the OpenAI API. If a request doesn't receive a response within this
	// timeframe, it will be terminated. The value is set to one minute,
	// which should be ample for most use cases.
	requestTimeout = 1 * time.Minute

	// responseMaxTokens sets the default maximum number of tokens to
	// generate for some OpenAI models like text generation. A token in OpenAI
	// API is typically equivalent to a word in English.
	responseMaxTokens = 128

	// responseTemperature sets the default "temperature" for sampling in
	// OpenAI models. It controls the randomness of the model's output. Higher
	// values (closer to 1) make output more random, while lower values (closer
	// to 0) make it more deterministic.
	responseTemperature = 0.5
)

// newWithStringParams creates a new OpenAI API client using simple parameters.
// Values are passed as strings in the sequence API Key, Organization ID,
// Parts of API Base URL...
func newWithStringParams(opts ...string) *Client {
	var apiKey, orgID, apiBaseURL string

	// Get apiKey and orgID from opts.
	heap := []*string{&apiKey, &orgID}
	for i := 0; i < len(opts) && i < len(heap); i++ {
		*heap[i] = opts[i]
	}

	// Get the base URL.
	// The baseURL can consist of many parts.
	if len(opts) > 2 {
		// Ignore the error here if the URL is invalid,
		// as the Client has an Error method of its own
		// that can detect it.
		opts = opts[2:]
		apiBaseURL, _ = urlBuild(opts[0], opts[1:]...)
	}

	return newWithConfigParams(Config{
		APIKey:     apiKey,
		OrgID:      orgID,
		APIBaseURL: apiBaseURL,
	})
}

// newWithConfigParams creates a new OpenAI API client using the provided
// configuration. This function is useful if you need to create a client
// with custom configurations, such as a specific context, HTTP headers,
// or HTTP client.
//
// Configuration are passed by value to protect against external modification.
// If no value is set for some parameters in the configuration, the default
// value will be used.
func newWithConfigParams(opts ...Config) *Client {
	cli := Client{}

	// Combine data from all transferred configurations.
	for _, opt := range opts {
		cli.Configure(opt)
	}

	return &cli
}

// New is a special constructor that creates a new OpenAI API client using
// simple parameters like apiKey, orgID, parts of the base URL sets as
// string values, or openai.Config type paramter(s).
//
// The first argument of the function is mandatory, it defines an API Key for
// simple parameter values, or a basic configuration of openai.Config type.
//
// The second and subsequent parameters must be of the same type as
// the first one.
//
// For simple parameters:
//   - second is an optional parameter, it is the organization ID;
//   - additional parameters, it is parts which form the base URL
//     for the OpenAI API (it can be useful for testing on your own
//     proxy, mock-server, etc.).
//
// Example usage:
//
//	// Create a new client with API Key only.
//	personalClient := openai.New("api-key")
//
//	// Create a new client with API Key and Organization ID.
//	organizationClient := openai.New("api-key", "org-id")
//
//	// Create a new client with APIKey and a custom base URL of API.
//	// Here, the organization ID is not specified, and the base URL of API
//	// can be composed of multiple parts.
//	v := "v1" // version
//	domain := "example.com"
//	customClient := openai.New("api-key", "", "https://", domain, v)
//
// For parameters of openai.Config type:
//   - second and next are optional parameters that extend
//     the value of the first.
//
// Configurations are passed by value to protect against external modification.
// If no value is set for some parameters in the configuration, the default
// value will be used.
//
// Example usage:
//
//	// Create a new client with APIKey and a custom context.
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	client := openai.New(openai.Config{
//	    APIKey: "api-key",
//	    Context: ctx,
//	})
//
//	// Or as two separate values.
//	client := openai.New(
//	    openai.Config{
//	        APIKey: "api-key",
//	    },
//	    openai.Config{
//	        Context: ctx,
//	    },
//	)
func New[T Config | string](v T, opts ...T) *Client {
	// The parameters are set as openai.Config type.
	if _, ok := any(v).(Config); ok {
		var data []Config = make([]Config, 0, len(opts)+1)
		data = append(data, any(v).(Config))

		for _, opt := range opts {
			data = append(data, any(opt).(Config))
		}

		return newWithConfigParams(data...)
	}

	// The parameters are set as string type.
	var data []string = make([]string, 0, len(opts)+1)
	data = append(data, any(v).(string))

	for _, opt := range opts {
		data = append(data, any(opt).(string))
	}

	return newWithStringParams(data...)
}
