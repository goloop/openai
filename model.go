package openai

// ModelDeleteResponse represents the response from the delete model endpoint.
type ModelDeleteResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Deleted bool   `json:"deleted"`
}

// ModelPermission represents a permission object.
type ModelPermission struct {
	// A unique identifier for the permission.
	ID string `json:"id"`

	// The type of the object, in this case, "model_permission".
	Object string `json:"object"`

	// The UNIX timestamp representing when the permission was created.
	Created int64 `json:"created"`

	// A boolean indicating if creating a new engine is allowed.
	AllowCreateEngine bool `json:"allow_create_engine"`

	// A boolean indicating if sampling is allowed.
	AllowSampling bool `json:"allow_sampling"`

	// A boolean indicating if log probabilities can be accessed.
	AllowLogprobs bool `json:"allow_logprobs"`

	// A boolean indicating if search indices can be used.
	AllowSearchIndices bool `json:"allow_search_indices"`

	// A boolean indicating if the model can be viewed.
	AllowView bool `json:"allow_view"`

	// A boolean indicating if the model can be fine-tuned.
	AllowFineTuning bool `json:"allow_fine_tuning"`

	// The organization to which the permission applies.
	Organization string `json:"organization"`

	// The group within the organization to which the permission applies.
	Group interface{} `json:"group"`

	// A boolean indicating if the permission is blocking.
	IsBlocking bool `json:"is_blocking"`
}

// ModelDetails represents a model object.
type ModelDetails struct {
	// A unique identifier for the model.
	ID string `json:"id"`

	// The type of the object, in this case, "model".
	Object string `json:"object"`

	// The UNIX timestamp representing when the model was created.
	Created int64 `json:"created"`

	// The owner of the model.
	OwnedBy string `json:"owned_by"`

	// A list of permissions associated with the model.
	Permission []ModelPermission `json:"permission"`

	// The root model from which the current model is derived.
	Root string `json:"root"`

	// The immediate parent model, if any,
	// from which the current model is derived.
	Parent interface{} `json:"parent"`
}

// ModelsData represents a list of models.
type ModelsData []*ModelDetails

// ModelResponse represents the response from the models endpoint.
type ModelResponse struct {
	// The type of the object, in this case, "list".
	Object string `json:"object"`

	// A list of Model objects, representing the available models.
	Data ModelsData `json:"data"`
}

// Name returns the name of the model.
func (ms *ModelDetails) Name() string {
	return ms.ID
}

// Range returns the models list.
func (data *ModelsData) Range() ModelsData {
	return *data
}

// Len returns the length of the models list.
func (data *ModelsData) Len() int {
	return len(*data)
}

// Names returns a list of the names of the models.
func (data *ModelsData) Names() []string {
	names := make([]string, data.Len())
	for i, m := range *data {
		names[i] = m.Name()
	}
	return names
}
