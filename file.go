package openai

import "os"

// Check if FileUploadRequest implements Requester interface.
var _ Requester = (*FileUploadRequest)(nil)

// FileDeleteResponse represents the response from the OpenAI File API
// when a file deletion request is made.
type FileDeleteResponse struct {
	// The unique identifier of the file that was deleted.
	ID string `json:"id"`

	// Object type - always "file" for this data type.
	Object string `json:"object"`

	// Indicates whether the file was successfully deleted.
	Deleted bool `json:"deleted"`
}

// FileDetails represents a single file's data in the response.
type FileDetails struct {
	// The unique identifier of the file.
	ID string `json:"id"`

	// Object type - always "file" for this data type.
	Object string `json:"object"`

	// The size of the file in bytes.
	Bytes int `json:"bytes"`

	// The time when the file was created, in Unix time.
	CreatedAt int `json:"created_at"`

	// The filename of the uploaded file.
	Filename string `json:"filename"`

	// The purpose of the file.
	Purpose string `json:"purpose"`
}

type FilesData []*FileDetails

// FileResponse represents the response from the OpenAI File API.
type FileResponse struct {
	// List of files belonging to the user's organization.
	Data FilesData `json:"data"`

	// Object type - always "list" for this type of response.
	Object string `json:"object"`
}

// FileUploadRequest represents a request to the OpenAI File API for
// uploading a file. This structure is used to provide the file and
// the purpose of the file to the API.
type FileUploadRequest struct {
	// Name of the JSON Lines file to be uploaded. This is a required field.
	File *os.File `json:"file"`

	// The intended purpose of the uploaded documents.
	// This is a required field. "fine-tune" is used for Fine-tuning.
	Purpose string `json:"purpose"`
}

// FileUploadResponse represents the response from the OpenAI File API
// when a file upload request is made. It provides information about
// the uploaded file.
type FileUploadResponse struct {
	// The unique identifier of the uploaded file.
	ID string `json:"id"`

	// Object type - always "file" for this data type.
	Object string `json:"object"`

	// The size of the uploaded file in bytes.
	Bytes int `json:"bytes"`

	// The Unix timestamp (seconds since the epoch) at
	// which the file was created.
	CreatedAt int `json:"created_at"`

	// The name of the uploaded file.
	Filename string `json:"filename"`

	// The intended purpose of the uploaded file. "fine-tune"
	// is used for Fine-tuning.
	Purpose string `json:"purpose"`
}

// Name returns the name of the file.
func (fd *FileDetails) Name() string {
	return fd.Filename
}

// Range returns the files list.
func (data *FilesData) Range() FilesData {
	return *data
}

// Len returns the length of the files list.
func (data *FilesData) Len() int {
	return len(*data)
}

// Names returns a list of the names of the files.
func (data *FilesData) Names() []string {
	names := make([]string, data.Len())
	for i, m := range *data {
		names[i] = m.Name()
	}
	return names
}

func (fr *FileUploadRequest) Error() error {
	if fr.File == nil {
		return ErrFileRequired
	}

	if fr.Purpose == "" {
		return ErrPurposeRequired
	}

	return nil
}

// OpenFile reads an any file from the provided path and assigns
// the *os.File value to the File field of the request.
func (r *FileUploadRequest) OpenFile(path string) error {
	r.CloseFile()
	file, err := os.Open(path)
	if err != nil {
		return err
	}

	// Assign the opened file to the File field of the request.
	r.File = file
	return nil
}

// CloseFile closes the file associated with the request.
func (r *FileUploadRequest) CloseFile() {
	if r.File != nil {
		r.File.Close()
	}
}

// Flush closes the files descriptors associated with the request.
func (r *FileUploadRequest) Flush() {
	r.CloseFile()
}
