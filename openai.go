package openai

import (
	"runtime"
	"time"
)

// The parallelTasks the default number of parallel requests.
var parallelTasks = runtime.NumCPU() * 2

const (
	// The apiBaseURL is the default base URL for the OpenAI API.
	apiBaseURL = "https://api.openai.com/v1"

	// Is the default maximum duration time for a request.
	requestTimeout = 1 * time.Minute

	// The default maximum number of tokens to generate.
	responseMaxTokens = 128

	// The default temperature for sampling.
	responseTemperature = 0.5
)

// NewClient creates a new OpenAI API client by simple parameters.
func NewClient(apiKey string, options ...string) *Client {
	var orgID, apiBaseURL string

	// Get the organization ID.
	if len(options) > 0 {
		orgID = options[0]
		options = options[1:]
	}

	// Get the base URL.
	// The baseURL can consist of many parts.
	if len(options) > 0 {
		// Ignore the error here if the URL is invalid,
		// as the Client has an Error method of its own
		// that can detect it.
		apiBaseURL, _ = urlBuild(options[0], options[1:]...)
	}

	return New(&Config{
		APIKey:     apiKey,
		OrgID:      orgID,
		APIBaseURL: apiBaseURL,
	})
}

// New creates a new OpenAI API client by the specified configuration.
func New(config *Config) *Client {
	cli := Client{
		apiKey: config.APIKey,
		orgID:  config.OrgID,
	}

	cli.Configure(config)
	return &cli
}
