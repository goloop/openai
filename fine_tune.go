package openai

// Check if FineTuneRequest implements Requester interface.
var _ Requester = (*FineTuneRequest)(nil)

// FineTuneRequest represents the request for a fine-tuning job.
type FineTuneRequest struct {
	TrainingFile                 string    `json:"training_file"`                            // ID of uploaded file with training data
	ValidationFile               string    `json:"validation_file,omitempty"`                // ID of uploaded file with validation data
	Model                        string    `json:"model,omitempty"`                          // Base model to fine-tune
	NEpochs                      int       `json:"n_epochs,omitempty"`                       // Number of epochs for training
	BatchSize                    int       `json:"batch_size,omitempty"`                     // Batch size for training
	LearningRateMultiplier       float64   `json:"learning_rate_multiplier,omitempty"`       // Multiplier for the learning rate
	PromptLossWeight             float64   `json:"prompt_loss_weight,omitempty"`             // Weight for loss on prompt tokens
	ComputeClassificationMetrics bool      `json:"compute_classification_metrics,omitempty"` // If true, calculates classification-specific metrics
	ClassificationNClasses       int       `json:"classification_n_classes,omitempty"`       // Number of classes in a classification task
	ClassificationPositiveClass  string    `json:"classification_positive_class,omitempty"`  // Positive class in binary classification
	ClassificationBetas          []float64 `json:"classification_betas,omitempty"`           // F-beta scores at the specified beta values
	Suffix                       string    `json:"suffix,omitempty"`                         // Suffix for the fine-tuned model name
}

// FineTuneEvent represents an event of a fine-tuning job.
type FineTuneEvent struct {
	Object    string `json:"object"`     // Object type (should be "fine-tune-event")
	CreatedAt int64  `json:"created_at"` // Timestamp of the event creation
	Level     string `json:"level"`      // Level of the event (for example, "info")
	Message   string `json:"message"`    // Message associated with the event
}

// Hyperparameters represents the hyperparameters used for fine-tuning.
type Hyperparameters struct {
	BatchSize              int     `json:"batch_size"`               // Batch size used for training
	LearningRateMultiplier float64 `json:"learning_rate_multiplier"` // Multiplier for the learning rate
	NEpochs                int     `json:"n_epochs"`                 // Number of epochs for training
	PromptLossWeight       float64 `json:"prompt_loss_weight"`       // Weight for loss on prompt tokens
}

// TrainingFile represents an uploaded file.
type TrainingFile struct {
	ID        string `json:"id"`         // ID of the file
	Object    string `json:"object"`     // Object type (should be "file")
	Bytes     int    `json:"bytes"`      // Size of the file in bytes
	CreatedAt int64  `json:"created_at"` // Timestamp of the file upload
	Filename  string `json:"filename"`   // Name of the file
	Purpose   string `json:"purpose"`    // Purpose of the file (for example, "fine-tune-train")
}

// FineTuneResponse represents the response for a fine-tuning job.
type FineTuneResponse struct {
	ID              string          `json:"id"`               // ID of the fine-tune task
	Object          string          `json:"object"`           // Object type (should be "fine-tune")
	Model           string          `json:"model"`            // Base model used for fine-tuning
	CreatedAt       int64           `json:"created_at"`       // Timestamp of the task creation
	Events          []FineTuneEvent `json:"events"`           // Events associated with the task
	FineTunedModel  *string         `json:"fine_tuned_model"` // Fine-tuned model name (null if not yet completed)
	Hyperparams     Hyperparameters `json:"hyperparams"`      // Hyperparameters used for fine-tuning
	OrganizationID  string          `json:"organization_id"`  // ID of the organization performing the fine-tuning
	ResultFiles     []interface{}   `json:"result_files"`     // Files with the results (empty if not yet completed)
	Status          string          `json:"status"`           // Status of the fine-tuning task
	ValidationFiles []interface{}   `json:"validation_files"` // Validation files (empty if not provided)
	TrainingFiles   []TrainingFile  `json:"training_files"`   // Training files
	UpdatedAt       int64           `json:"updated_at"`       // Timestamp of the last update
}

type FineTunesData []*FineTuneResponse

// FineTuneListResponse represents a list of fine-tuning jobs.
type FineTuneListResponse struct {
	Object string        `json:"object"` // Object type (should be "list")
	Data   FineTunesData `json:"data"`   // List of fine-tuning jobs
}

type FineTuneEventsData []*FineTuneEvent

// FineTuneEventListResponse represents the response from the OpenAI
// API when requesting fine-tuning job events. It contains a list
// of fine-tuning job events.
type FineTuneEventListResponse struct {
	Object string             `json:"object"` // Type of the object (list)
	Data   FineTuneEventsData `json:"data"`   // List of fine-tuning job events
}

// Error returns an error if the request is invalid.
func (ftr *FineTuneRequest) Error() error {
	return nil
}

// Flush does nothing.
// It here to implement the Requester interface.
func (ftr *FineTuneRequest) Flush() {
}
