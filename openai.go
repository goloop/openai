package openai

import "github.com/goloop/ai"

// DefaultBaseURL is the OpenAI API base URL, including the version segment.
const DefaultBaseURL = "https://api.openai.com/v1"

// Convenience model identifiers. Any model string is accepted; use Models to
// discover what the account can call.
const (
	ModelGPT4o     = "gpt-4o"
	ModelGPT4oMini = "gpt-4o-mini"
	ModelGPT4Turbo = "gpt-4-turbo"
	ModelO3Mini    = "o3-mini"
)

// Client is an OpenAI API client. It implements [ai.Client] and adds the
// provider's native endpoints.
type Client struct {
	opts    ai.Options
	orgID   string
	project string
}

var _ ai.Client = (*Client)(nil)

// New returns a Client for the given API key. Shared options (WithBaseURL,
// WithHTTPClient, WithTimeout, WithMaxRetries, WithHeader) and OpenAI options
// (WithOrg, WithProject) configure it.
func New(apiKey string, opts ...Option) *Client {
	s := settings{}
	for _, o := range opts {
		o(&s)
	}

	o := ai.NewOptions(apiKey, s.aiOpts...)
	if o.BaseURL == "" {
		o.BaseURL = DefaultBaseURL
	}

	return &Client{opts: o, orgID: s.orgID, project: s.project}
}
