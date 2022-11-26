package toolkit

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const gigabyte = 1024 * 1024 * 1024
const megabyte = 1024 * 1024

type Tools struct {
	MaxJSONSize        int
	AllowUnknownFields bool
	MaxFileSize        int
	AllowedFileTypes   []string
}

func createRandomStringSource() string {
	lowerAlphaCharacters := loweralpha()
	specials := "~!@#$"
	return lowerAlphaCharacters + strings.ToUpper(lowerAlphaCharacters) + zerototen() + specials
}

// random string helper functions
// ---
// generate lower case alphabet
func loweralpha() string {
	p := make([]byte, 26)
	for i := range p {
		p[i] = 'a' + byte(i)
	}
	return string(p)
}

// generate integers 0-9 as string
func zerototen() string {
	var ints []string
	for i := 0; i < 10; i++ {
		ints = append(ints, strconv.Itoa(i))
	}
	return strings.Join(ints, "")
}

// RandomString returns a string of random characters of length n
// uses randomStringSource as the source for the string
func (t *Tools) RandomString(n int) string {
	sliceOfRunes, randomRunes := make([]rune, n), []rune(createRandomStringSource())
	for idx := range sliceOfRunes {
		primeNumber, _ := rand.Prime(rand.Reader, len(randomRunes))
		x, y := primeNumber.Uint64(), uint64(len(randomRunes))
		sliceOfRunes[idx] = randomRunes[x%y]
	}
	return string(sliceOfRunes)
}

// UploadedFile is a struct used to save information about an uploaded file
type UploadedFile struct {
	NewFileName      string
	OriginalFileName string
	FileSize         int64
}

// CreateDirectoryIfNotExist creates a directory, and all necessary parents, if it does not exist
func (tools *Tools) CreateDirectoryIfNotExist(path string) error {
	const mode = 0755
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.MkdirAll(path, mode)
		if err != nil {
			return err
		}
	}
	return nil
}

// CreateSlug is simplified means of creating a slug from a string
func (tools *Tools) CreateSlug(str string) (string, error) {
	slug := ""
	if str == "" {
		return slug, errors.New("empty string not permitted")
	}

	var regEx = regexp.MustCompile(`[^[a-z\d]+`)
	slug = strings.Trim(regEx.ReplaceAllString(strings.ToLower(str), "-"), "-")
	if len(slug) == 0 {
		return "", errors.New("after removing characters, slug is zero length")
	}
	return slug, nil
}

// DownloadStaticFile downloads a file and tries to force download to avoid displaying it
// sets Content-Disposition
// allows display name specification
func (tools *Tools) DownloadStaticFile(responseWriter http.ResponseWriter, request *http.Request, pth, file, displayName string) {
	filePath := path.Join(pth, file)
	responseWriter.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", displayName))
	http.ServeFile(responseWriter, request, filePath)
}

// UploadOneFile handles a single file passed in multipart form request
func (tools *Tools) UploadOneFile(request *http.Request, uploadDirectory string, rename ...bool) (*UploadedFile, error) {
	renameFile := true
	if len(rename) > 0 {
		renameFile = rename[0]
	}
	files, err := tools.UploadFiles(request, uploadDirectory, renameFile)
	if err != nil {
		return nil, err
	}

	return files[0], nil
}

// UploadFiles can handle multiple files in multipart form request
func (tools *Tools) UploadFiles(request *http.Request, uploadDirectory string, rename ...bool) ([]*UploadedFile, error) {

	// default to renameFile true
	// if rename provided in arguments
	// take 0 index as that should hold the boolean value to set renameFile
	renameFile := true
	if len(rename) > 0 {
		renameFile = rename[0]
	}

	var uploadedFiles []*UploadedFile

	if tools.MaxFileSize == 0 {
		// set default size
		tools.MaxFileSize = gigabyte
	}
	// create uploadDirectory if it doesn't exist
	err := tools.CreateDirectoryIfNotExist(uploadDirectory)
	if err != nil {
		return nil, err
	}
	// parse multipart form data and throw error if request size is too large
	err = request.ParseMultipartForm(int64(tools.MaxFileSize))
	if err != nil {
		return nil, errors.New("the uploaded file is too big")
	}

	for _, fileHeaders := range request.MultipartForm.File {
		for _, header := range fileHeaders {
			uploadedFiles, err = func(uploadedFiles []*UploadedFile) ([]*UploadedFile, error) {
				var uploadedFile UploadedFile
				infile, err := header.Open()

				if err != nil {
					return nil, err
				}
				defer infile.Close()

				first512bytesBuffer := make([]byte, 512)

				_, err = infile.Read(first512bytesBuffer)
				if err != nil {
					return nil, err
				}

				// check to see if the file type is permitted
				allowed := false
				fileType := http.DetectContentType(first512bytesBuffer)

				if len(tools.AllowedFileTypes) > 0 {
					for _, allowedType := range tools.AllowedFileTypes {
						// EqualFold compares two strings while ignoring case
						if strings.EqualFold(fileType, allowedType) {
							allowed = true
						}
					}
				} else {
					// if no allowed file types configured, assume all types are allowed
					allowed = true
				}

				if !allowed {
					return nil, errors.New("uploaded file type is not permitted")
				}

				_, err = infile.Seek(0, 0)
				if err != nil {
					return nil, err
				}
				if renameFile {
					uploadedFile.NewFileName = fmt.Sprintf("%d_%s%s", time.Now().Unix(), tools.RandomString(20), filepath.Ext(header.Filename))
				} else {
					uploadedFile.NewFileName = header.Filename
				}
				uploadedFile.OriginalFileName = header.Filename
				var outfile *os.File
				defer outfile.Close()

				if outfile, err = os.Create(filepath.Join(uploadDirectory, uploadedFile.NewFileName)); err != nil {
					return nil, err
				} else {
					fileSize, err := io.Copy(outfile, infile)
					if err != nil {
						return nil, err
					}
					uploadedFile.FileSize = fileSize
				}

				uploadedFiles = append(uploadedFiles, &uploadedFile)
				return uploadedFiles, nil
			}(uploadedFiles)
			if err != nil {
				return uploadedFiles, err
			}
		}
	}
	return uploadedFiles, nil
}

// JSONResponse is Type used for sending JSON
type JSONResponse struct {
	Error   bool        `json:"error"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ReadJSON tries to read the body of a request and converts from json into a go data variable
func (tools *Tools) ReadJSON(responseWriter http.ResponseWriter, request *http.Request, data interface{}) error {
	maxBytes := megabyte // one megabyte
	if tools.MaxJSONSize != 0 {
		maxBytes = tools.MaxJSONSize
	}

	request.Body = http.MaxBytesReader(responseWriter, request.Body, int64(maxBytes))

	jsonDecoder := json.NewDecoder(request.Body)

	if !tools.AllowUnknownFields {
		jsonDecoder.DisallowUnknownFields()
	}

	err := jsonDecoder.Decode(data)

	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError

		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset)
		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly-formed JSON")
		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
			}
			return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)
		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")
		case strings.HasPrefix(err.Error(), "json: unknown field"):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field")
			return fmt.Errorf("body contains unknown key %s", fieldName)
		case err.Error() == "http: request body too large":
			return fmt.Errorf("body must not be larger than %d bytes", maxBytes)
		case errors.As(err, &invalidUnmarshalError):
			return fmt.Errorf("error unmarshalling JSON: %s", err.Error())
		default:
			return err
		}
	}

	err = jsonDecoder.Decode(&struct{}{})

	if err != io.EOF {
		return errors.New("body must contain only one JSON value")
	}

	return nil
}

// WriteJSON accepts a response status code and arbitrary data and writes JSON to the client
func (tools *Tools) WriteJSON(responseWriter http.ResponseWriter, status int, data interface{}, headers ...http.Header) error {
	output, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if len(headers) > 0 {
		for key, value := range headers[0] {
			responseWriter.Header()[key] = value
		}
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(status)
	_, err = responseWriter.Write(output)
	if err != nil {
		return err
	}
	return nil
}

// ErrorJSON takes an error and optionally a status code
// generates and sends a JSON error message
func (tools *Tools) ErrorJSON(responseWriter http.ResponseWriter, err error, status ...int) error {
	statusCode := http.StatusBadRequest

	if len(status) > 0 {
		statusCode = status[0]
	}

	var payload JSONResponse
	payload.Error = true
	payload.Message = err.Error()

	return tools.WriteJSON(responseWriter, statusCode, payload)
}

// SendJSONToRemote expects standard url.URL, arbitrary data, and an optional http Client
// If no http client is specified, standard http.Client is used
func (tools *Tools) PostJSONToRemote(uri url.URL, data interface{}, client ...*http.Client) (*http.Response, int, error) {
	// create JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, 0, err
	}

	// check for custom http client
	httpClient := &http.Client{}
	if len(client) > 0 {
		httpClient = client[0]
	}

	// build request
	request, err := http.NewRequest("POST", uri.String(), bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, 0, err
	}

	// set header
	request.Header.Set("Content-Type", "application/json")

	// call remote URI
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()

	// send response back
	return response, response.StatusCode, nil
}
