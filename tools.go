package openai

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

// The toImagePath modifies the image path to reflect the copy number
// and additional suffixes if provided. It resolves relative paths,
// and replaces ~ with the home directory path. If the provided path
// is a directory, it generates a unique filename with a .png extension.
func toImagePath(copy int, path string, sep ...string) (string, error) {
	// Resolve ~ to the user's home directory.
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[2:])
	}

	// Resolve relative paths to absolute paths.
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	if strings.HasSuffix(absolutePath, "/") ||
		strings.HasSuffix(absolutePath, "\\") ||
		!strings.HasSuffix(absolutePath, ".png") {

		// Check if the provided path is a directory
		info, err := os.Stat(absolutePath)
		if err != nil {
			return "", err
		}

		if info.IsDir() {
			// If the path is a directory, generate a unique filename
			file, err := generateUniqueFilename()
			if err != nil {
				return "", err
			}
			return filepath.Join(absolutePath, file+".png"), nil
		}
	}

	// Get file extension.
	ext := filepath.Ext(absolutePath)

	// File name without extension.
	file := strings.TrimSuffix(filepath.Base(absolutePath), ext)

	// File path without file name.
	dir := filepath.Dir(absolutePath)

	if len(sep) == 0 {
		sep = []string{"_"}
	}

	// Build a new name with optional separators
	for _, s := range sep {
		file += s
	}

	if copy != 0 {
		file += strconv.Itoa(copy)
	}

	file += ext

	return filepath.Join(dir, file), nil
}

// The generateUniqueFilename generates a unique filename using
// the crypto/rand package from the Go standard library.
//
// Don't to use third-party libraries, for example github.com/google/uuid.
func generateUniqueFilename() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	uuid := fmt.Sprintf("%x", b)
	return uuid, nil
}

// saveByURL is a function that saves images from a list of URLs to the
// specified path on the local filesystem. It takes the path to save the
// images, the number of parallel tasks to execute, and a slice of URLs
// as input.
// It returns an error if there was any issue during the process.
func saveByURL(path string, parallelTasks int, items []string) error {
	var wg sync.WaitGroup
	var errors []error
	var errMutex sync.Mutex

	// Create a semaphore with a maximum count of parallelTasks.
	sem := make(chan struct{}, parallelTasks)

	for i, item := range items {
		// Increment waitgroup counter.
		wg.Add(1)

		// Acquire a token.
		sem <- struct{}{}

		go func(i int, item string) {
			// Release token when done.
			defer func() { <-sem; wg.Done() }()

			p, err := toImagePath(i, path)
			if err != nil {
				errMutex.Lock()
				errors = append(errors, err)
				errMutex.Unlock()
				return
			}

			resp, err := http.Get(item)
			if err != nil {
				errMutex.Lock()
				errors = append(errors, err)
				errMutex.Unlock()
				return
			}
			defer resp.Body.Close()

			out, err := os.Create(p)
			if err != nil {
				errMutex.Lock()
				errors = append(errors, err)
				errMutex.Unlock()
				return
			}
			defer out.Close()

			_, err = io.Copy(out, resp.Body)
			if err != nil {
				errMutex.Lock()
				errors = append(errors, err)
				errMutex.Unlock()
				return
			}
		}(i, item)
	}

	// Wait for all goroutines to finish.
	wg.Wait()

	if len(errors) > 0 {
		return errors[0]
	}

	return nil
}

// saveByBase64 is a function that saves images from a list of
// base64-encoded strings to the specified path on the local filesystem.
// It takes the path to save the images, the number of parallel
// tasks to execute, and a slice of base64-encoded strings as input.
// It returns an error if there was any issue during the process.
func saveByBase64(path string, parallelTasks int, items []string) error {
	var wg sync.WaitGroup
	var errors []error
	var errMutex sync.Mutex

	// Create a semaphore with a maximum count of parallelTasks.
	sem := make(chan struct{}, parallelTasks)

	for i, item := range items {
		// Increment waitgroup counter.
		wg.Add(1)

		// Acquire a token.
		sem <- struct{}{}

		go func(i int, item string) {
			// Release token when done.
			defer func() { <-sem; wg.Done() }()

			// Convert base64 to bytes.
			dec, err := base64.StdEncoding.DecodeString(item)
			if err != nil {
				errMutex.Lock()
				errors = append(errors, err)
				errMutex.Unlock()
				return
			}

			p, err := toImagePath(i, path)
			if err != nil {
				errMutex.Lock()
				errors = append(errors, err)
				errMutex.Unlock()
				return
			}

			// Write bytes to file.
			err = ioutil.WriteFile(p, dec, 0o644)
			if err != nil {
				errMutex.Lock()
				errors = append(errors, err)
				errMutex.Unlock()
				return
			}
		}(i, item)
	}

	// Wait for all goroutines to finish.
	wg.Wait()

	if len(errors) > 0 {
		return errors[0]
	}

	return nil
}

// The urlBuild constructs a URL from a base URL as prefix
// (like: https://some.site/) and an endpoint (or path parts).
// The function returns an error as the second value if the URL
// is invalid.
func urlBuild(baseURL string, pathParts ...string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	u.Path = path.Join(u.Path, path.Join(pathParts...))
	return u.String(), nil
}

// The isSuccessfulCode checks if the HTTP status code is successful.
func isSuccessfulCode(statusCode int) bool {
	// Different endpoints has different successful status code,
	// it can be: 200, 202, 204 etc.
	return statusCode >= http.StatusOK && statusCode < http.StatusBadRequest
}

// newJSONRequest creates a new HTTP request instance.
func newJSONRequest(c Clienter, m, u string, b any) (*http.Request, error) {
	var body io.Reader

	// Create a request body if it is passed.
	if b != nil {
		tmp, err := json.Marshal(b)
		if err != nil {
			return &http.Request{}, err
		}
		body = bytes.NewBuffer(tmp)
	}

	// Create a new HTTP request.
	req, err := http.NewRequestWithContext(c.Context(), m, u, body)
	if err != nil {
		return req, err
	}

	// Set the request headers.
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey()))
	if orgID := c.OrgID(); orgID != "" {
		req.Header.Set("OpenAI-Organization", orgID)
	}

	// Add additional headers.
	for k, values := range c.HTTPHeaders() {
		for _, v := range values {
			req.Header.Add(k, v)
		}
	}

	return req, nil
}

// newDataRequest is a helper function that creates a new
// multipart/form-data HTTP request.
// It takes a Clienter interface, HTTP method, URL, and request body as input.
// It uses reflection to iterate over the fields of the request body and
// construct the form data. The function supports file uploads by creating
// form files for *os.File fields. It returns the constructed HTTP request
// or an error if there was any issue during the process.
func newDataRequest(c Clienter, m, u string, b any) (*http.Request, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Use reflection to get the Value and Type of the request
	// Indirect handles both pointers and values
	val := reflect.Indirect(reflect.ValueOf(b))
	typ := val.Type()

	// Iterate over the fields of the request
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		typeField := typ.Field(i)
		tag := typeField.Tag

		// Skip fields without json tag
		jsonTag := tag.Get("json")
		if jsonTag == "" {
			continue
		}

		// Split the tag and use the first part (before omitempty, if present).
		jsonFieldName := strings.Split(jsonTag, ",")[0]

		if field.Type().String() == "*os.File" {
			file, ok := field.Interface().(*os.File)
			if ok && file != nil {
				fieldWriter, err := writer.CreateFormFile(
					jsonFieldName,
					filepath.Base(file.Name()),
				)
				if err != nil {
					return &http.Request{}, err
				}

				_, err = io.Copy(fieldWriter, file)
				if err != nil {
					return &http.Request{}, err
				}
			}
		} else {
			// Check if the field is of type string.
			if field.Kind() == reflect.String {
				err := writer.WriteField(jsonFieldName, field.String())
				if err != nil {
					return &http.Request{}, err
				}
			} else {
				jsonField, err := json.Marshal(field.Interface())
				if err != nil {
					return &http.Request{}, err
				}

				err = writer.WriteField(jsonFieldName, string(jsonField))
				if err != nil {
					return &http.Request{}, err
				}
			}
		}
	}

	err := writer.Close()
	if err != nil {
		return &http.Request{}, err
	}

	req, err := http.NewRequestWithContext(c.Context(), m, u, body)
	if err != nil {
		return &http.Request{}, err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey()))
	if orgID := c.OrgID(); orgID != "" {
		req.Header.Set("OpenAI-Organization", orgID)
	}

	return req, err
}

// The doRequest performs an HTTP request and returns
// the response body as a byte slice.
func doRequest(
	c Clienter,
	req *http.Request,
	goal any,
) ([]byte, error) {
	// Send request.
	resp, err := c.HTTPClient().Do(req)
	if err != nil {
		netErr, ok := err.(net.Error)
		if ok && netErr.Timeout() {
			return []byte{}, ErrRequestTimedOut
		}
		return []byte{}, err
	}
	defer resp.Body.Close()

	// Check the HTTP status code.
	if !isSuccessfulCode(resp.StatusCode) {
		// Read the response errorBody.
		errorBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return []byte{}, fmt.Errorf("failed to read error body: %v", err)
		}

		errorResponse := ErrorResponse{}
		json.Unmarshal(errorBody, &errorResponse)

		// Return an error that includes the status code and the error details.
		return []byte{}, fmt.Errorf(
			"non-success status code %d: %s",
			resp.StatusCode,
			errorResponse.Error.Message,
		)
	}

	// Read response body.
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, err
	}

	// Unmarshal response body if goal is not nil and is a pointer to a struct.
	if goal != nil &&
		reflect.ValueOf(goal).Kind() == reflect.Ptr &&
		reflect.Indirect(reflect.ValueOf(goal)).Kind() == reflect.Struct {
		err = json.Unmarshal(body, goal)
		if err != nil {
			return []byte{}, err
		}
	}

	return body, nil
}
