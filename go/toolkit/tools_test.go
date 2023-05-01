package toolkit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"testing"

	"github.com/fatih/color"
)

func TestTools_RandomString(test *testing.T) {
	var testTools Tools

	str := testTools.RandomString(10)

	if len(str) != 10 {
		test.Error("Incorrect length of random string returned")
	}
}

var uploadTests = []struct {
	name          string
	allowedTypes  []string
	renameFile    bool
	errorExpected bool
}{
	{name: "allowed no rename", allowedTypes: []string{"image/jpeg", "image/png"}, renameFile: false, errorExpected: false},
	{name: "allowed and rename", allowedTypes: []string{"image/jpeg", "image/png"}, renameFile: true, errorExpected: false},
	{name: "not allowed", allowedTypes: []string{"image/png"}, renameFile: false, errorExpected: true},
}

func TestTools_UploadFiles(test *testing.T) {
	for _, entry := range uploadTests {
		// set up a pipe to avoid buffering
		pipeReader, pipeWriter := io.Pipe()
		writer := multipart.NewWriter(pipeWriter)
		waitGroup := sync.WaitGroup{}
		waitGroup.Add(1)

		go func() {
			defer writer.Close()
			defer waitGroup.Done()

			// create the form data field "file"
			part, err := writer.CreateFormFile("file", "./testdata/test_image.jpg")
			if err != nil {
				test.Error(err)
			}

			newFile, err := os.Open("./testdata/test_image.jpg")
			if err != nil {
				test.Error(err)
			}

			defer newFile.Close()

			img, _, err := image.Decode(newFile)
			if err != nil {
				test.Error("error decoding image", err)
			}
			err = jpeg.Encode(part, img, &jpeg.Options{Quality: 100})
			if err != nil {
				test.Error(err)
			}
		}()

		// read from the pipe which recieves data
		request := httptest.NewRequest("POST", "/", pipeReader)
		request.Header.Add("Content-Type", writer.FormDataContentType())
		var testTools Tools
		testTools.AllowedFileTypes = entry.allowedTypes
		uploadedFiles, err := testTools.UploadFiles(request, "./testdata/uploads/", entry.renameFile)
		if err != nil && !entry.errorExpected {
			test.Error(err)
		}
		if !entry.errorExpected {
			if _, err := os.Stat(fmt.Sprintf("./testdata/uploads/%s", uploadedFiles[0].NewFileName)); os.IsNotExist(err) {
				test.Errorf("%s: expected file to exist: %s", entry.name, err.Error())
			}

			// clean up
			_ = os.Remove(fmt.Sprintf("./testdata/uploads/%s", uploadedFiles[0].NewFileName))
		}

		if !entry.errorExpected && err != nil {
			test.Errorf("%s: error expected but non received", entry.name)
		}

		waitGroup.Wait()
	}
}

func TestTools_UploadOneFile(test *testing.T) {
	// set up a pipe to avoid buffering
	pipeReader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)

	go func() {
		defer writer.Close()

		// create the form data field "file"
		part, err := writer.CreateFormFile("file", "./testdata/test_image.jpg")
		if err != nil {
			test.Error(err)
		}

		newFile, err := os.Open("./testdata/test_image.jpg")
		if err != nil {
			test.Error(err)
		}

		defer newFile.Close()

		img, _, err := image.Decode(newFile)
		if err != nil {
			test.Error("error decoding image", err)
		}
		err = jpeg.Encode(part, img, &jpeg.Options{Quality: 100})
		if err != nil {
			test.Error(err)
		}
	}()

	// read from the pipe which recieves data
	request := httptest.NewRequest("POST", "/", pipeReader)
	request.Header.Add("Content-Type", writer.FormDataContentType())
	var testTools Tools

	uploadedFile, err := testTools.UploadOneFile(request, "./testdata/uploads/", true)
	if err != nil {
		test.Error(err)
	}

	if _, err := os.Stat(fmt.Sprintf("./testdata/uploads/%s", uploadedFile.NewFileName)); os.IsNotExist(err) {
		test.Errorf("%s: expected file to exist", err.Error())
	}

	// clean up
	_ = os.Remove(fmt.Sprintf("./testdata/uploads/%s", uploadedFile.NewFileName))

}

func TestTools_CreateDirectoryIfNotExist(test *testing.T) {
	var testTool Tools
	testDirectoryPath := "./testdata/myTestDir"

	err := testTool.CreateDirectoryIfNotExist(testDirectoryPath)
	if err != nil {
		test.Error(err)
	}
	err = testTool.CreateDirectoryIfNotExist(testDirectoryPath)
	if err != nil {
		test.Error(err)
	}

	_ = os.Remove(testDirectoryPath)
}

var slugTests = []struct {
	name          string
	str           string
	expected      string
	errorExpected bool
}{
	{name: "valid string", str: "now is the time", expected: "now-is-the-time", errorExpected: false},
	{name: "empty string", str: "", expected: "<nothing_expected>", errorExpected: true},
	{name: "valid string", str: "Now is the time for all GOOD people! + fish & such^123", expected: "now-is-the-time-for-all-good-people-fish-such-123", errorExpected: false},
	{name: "japanese string", str: "おはようございます", expected: "<nothing_expected>", errorExpected: true},
	{name: "japanese and roman character string", str: "Good Morning おはようございます", expected: "good-morning", errorExpected: false},
}

func TestTools_CreateSlug(test *testing.T) {
	var testTool Tools
	for _, entry := range slugTests {
		slug, err := testTool.CreateSlug(entry.str)
		if err != nil && !entry.errorExpected {
			test.Errorf("%s: error received when none expected: %s", entry.name, err.Error())
		}

		if !entry.errorExpected && slug != entry.expected {
			test.Errorf("%s: wrong slug returned; expected %s but got %s", entry.name, entry.expected, slug)
		}
	}
}

func TestTools_DownloadStaticFile(test *testing.T) {
	responseRecorder := httptest.NewRecorder()
	request, _ := http.NewRequest("GET", "/", nil)

	var testTools Tools

	testTools.DownloadStaticFile(responseRecorder, request, "./testdata", "test_image.jpg", "camping.jpg")
	response := responseRecorder.Result()
	defer response.Body.Close()

	responseContentLength := response.Header["Content-Length"][0]
	responseContentDisposition := response.Header["Content-Disposition"][0]
	testImageSize := "3582201"

	if responseContentLength != testImageSize {
		test.Error("wrong content length of", responseContentLength)
	}

	if responseContentDisposition != "attachment; filename=\"camping.jpg\"" {
		test.Error("wrong content disposition", responseContentDisposition)
	}

	_, err := ioutil.ReadAll(response.Body)
	if err != nil {
		test.Error(err)
	}
}

var jsonTests = []struct {
	name               string
	json               string
	errorExpected      bool
	maxSize            int
	allowUnknownFields bool
}{
	{name: "good json", json: `{"foo":"bar"}`, errorExpected: false, maxSize: 1024, allowUnknownFields: false},
	{name: "badly formatted json", json: `{"foo":}`, errorExpected: true, maxSize: 1024, allowUnknownFields: false},
	{name: "incorrect type", json: `{"foo": 1}`, errorExpected: true, maxSize: 1024, allowUnknownFields: false},
	{name: "two json files", json: `{"foo": "bar"}{"alpha":"beta"}`, errorExpected: true, maxSize: 1024, allowUnknownFields: false},
	{name: "empty body", json: `""`, errorExpected: true, maxSize: 1024, allowUnknownFields: false},
	{name: "syntax error in json", json: `{"foo": bar"`, errorExpected: true, maxSize: 1024, allowUnknownFields: false},
	{name: "unknown field in json", json: `{"food": "bar"}`, errorExpected: true, maxSize: 1024, allowUnknownFields: false},
	{name: "allow unknown fields in json", json: `{"food": "bar"}`, errorExpected: false, maxSize: 1024, allowUnknownFields: true},
	{name: "missing field name", json: `{jack: "bar"}`, errorExpected: true, maxSize: 1024, allowUnknownFields: false},
	{name: "file too large", json: `{"foo": "bar"}`, errorExpected: true, maxSize: 5, allowUnknownFields: true},
	{name: "not json", json: `"foo equals bar"`, errorExpected: true, maxSize: 1024, allowUnknownFields: true},
}

func TestTools_ReadJSON(test *testing.T) {
	var testTool Tools

	for _, entry := range jsonTests {
		// set max file size
		testTool.MaxJSONSize = entry.maxSize

		// allow/disallow unknown fields
		testTool.AllowUnknownFields = entry.allowUnknownFields

		// declare a variable to read the decoded json into
		var decodedJSON struct {
			Foo string `json:"foo"`
		}

		// create a request with the body
		request, err := http.NewRequest("POST", "/", bytes.NewReader([]byte(entry.json)))
		if err != nil {
			test.Log("Error: ", err)
		}
		defer request.Body.Close()
		// create a record
		responseRecorder := httptest.NewRecorder()
		err = testTool.ReadJSON(responseRecorder, request, &decodedJSON)
		// error expected but didn't get one
		if entry.errorExpected && err == nil {
			test.Errorf("%s: error expected, but none received", entry.name)
		}
		// error not expected, but received
		if !entry.errorExpected && err != nil {
			test.Errorf("%s: error not expected, but one received: %s", entry.name, err.Error())
		}
	}
}

func TestTools_WriteJSON(test *testing.T) {
	var testTools Tools

	responseRecorder := httptest.NewRecorder()
	payload := JSONResponse{
		Error:   false,
		Message: "foo",
	}

	headers := make(http.Header)
	headers.Add("FOO", "BAR")

	err := testTools.WriteJSON(responseRecorder, http.StatusOK, payload, headers)
	if err != nil {
		test.Errorf("failed to write JSON: %v", err)
	}
}

func TestTools_ErrorJSON(test *testing.T) {
	var testTools Tools
	responseRecorder := httptest.NewRecorder()
	err := testTools.ErrorJSON(responseRecorder, errors.New("some error"), http.StatusServiceUnavailable)
	if err != nil {
		test.Error(err)
	}

	var payload JSONResponse

	decoder := json.NewDecoder(responseRecorder.Body)
	err = decoder.Decode(&payload)
	if err != nil {
		test.Error("received error when decoding JSON", err)
	}

	if !payload.Error {
		test.Error("error set to false in JSON, and it should be true")
	}

	if responseRecorder.Code != http.StatusServiceUnavailable {
		test.Errorf("wrong status code returned; expected 503, but got %d", responseRecorder.Code)
	}
}

type RoundTripFunc func(req *http.Request) *http.Response

func (f RoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request), nil
}

func NewTestClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: fn,
	}
}

func TestTools_PostJSONToRemote(test *testing.T) {
	client := NewTestClient(func(request *http.Request) *http.Response {
		// Test request parameters
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(bytes.NewBufferString("ok")),
			Header:     make(http.Header),
		}
	})

	var testTools Tools
	var foo struct {
		Bar string `json:"bar"`
	}
	foo.Bar = "bar"

	remoteURI := url.URL{
		Scheme: "http",
		Host:   "example.com",
		Path:   "some/path",
	}
	_, _, err := testTools.PostJSONToRemote(remoteURI, foo, client)
	if err != nil {
		test.Error("failed to call remote url: ", err)
	}
}

func TestTools_CloseListener(test *testing.T) {
	var testTools Tools
	testTools.CloseListener(exitGracefully, "exit triggered!")
}

func exitGracefully(err error, msg ...string) {
	message := ""
	if len(msg) > 0 {
		message = msg[0]
	}
	if err != nil {
		color.Red("Error: %v\n", err)
	}

	if len(message) > 0 {
		color.Yellow(message)
	} else {
		color.Green("Finished")
	}

	os.Exit(0)
}
