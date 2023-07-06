[![Go Report Card](https://goreportcard.com/badge/github.com/goloop/openai)](https://goreportcard.com/report/github.com/goloop/openai) [![License](https://img.shields.io/badge/license-MIT-brightgreen)](https://github.com/goloop/openai/blob/master/LICENSE) [![License](https://img.shields.io/badge/godoc-YES-green)](https://godoc.org/github.com/goloop/openai) [![Stay with Ukraine](https://img.shields.io/static/v1?label=Stay%20with&message=Ukraine%20♥&color=ffD700&labelColor=0057B8&style=flat)](https://u24.gov.ua/)


# openai

Go clients for OpenAI API

**DO NOT USE THIS VERSION IN PRODUCTION, BECAUSE IT IS AN ALPHA VERSION**


# Examples

Import the appropriate library.

```go
import "github.com/goloop/openai"
```

# New client

A personal client with default settings in can be quickly created using the New function.

```go
apiKey := "sk-..."
client := openai.New(apiKey)
```

We can also add the Organization ID for the organization's client.

```go
apiKey := "sk-..."
orgID := "org-..."
client := openai.New(apiKey, orgID)
```

We can set our own base URL of API, which can consist of several parts.

```go
apiKey := "sk-..."
orgID := "org-..."
domain := "example.com"
v := "v1"
client := openai.New(apiKey, orgID, "https://", domain, v)

```

The client can be created with advanced configurations.

It is not necessary to specify all possible configuration parameters. Parameters not specified will be defined by default.

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

client := openai.New(openai.Config{
    APIKey:        "sk-...",
	OrgID:         "org-...",
    Context:       ctx,
    ParallelTasks: 8,
})
```

