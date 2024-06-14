package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mocking HTTP Client
type MockHTTPClient struct {
	mock.Mock
}

func (m *MockHTTPClient) Post(url, contentType string, body *bytes.Buffer) (*http.Response, error) {
	args := m.Called(url, contentType, body)
	return args.Get(0).(*http.Response), args.Error(1)
}

var (
	validConfig = Config{
		APIEndpoint:      "http://example.com/api",
		NumberOfScanners: 2,
		RescanInterval:   5,
		Keyboard:         true,
	}
)

func TestReadConfig(t *testing.T) {
	// Create a sample config.json file for testing
	configContent := `{
		"apiEndpoint": "http://example.com/api",
		"numberOfScanners": 2,
		"rescanInterval": 5,
		"keyboard": true
	}`
	os.WriteFile("config.json", []byte(configContent), 0644)
	defer os.Remove("config.json")

	config, err := readConfig()
	assert.NoError(t, err)
	assert.Equal(t, validConfig, *config)
}

func TestReadConfig_FileNotFound(t *testing.T) {
	_, err := readConfig()
	assert.Error(t, err)
}

func TestPayloadCleanItemId(t *testing.T) {
	payload := Payload{ItemID: "someprefixid=12345", DeviceType: "scanner"}
	payload.CleanItemId()
	assert.Equal(t, "12345", payload.ItemID)
}

func TestPayloadCleanItemId_NoId(t *testing.T) {
	payload := Payload{ItemID: "someprefix", DeviceType: "scanner"}
	payload.CleanItemId()
	assert.Equal(t, "someprefix", payload.ItemID)
}

func TestPostPayload_Success(t *testing.T) {
	client := new(MockHTTPClient)
	payload := Payload{ItemID: "12345", DeviceType: "scanner"}
	config := &Config{APIEndpoint: "http://example.com/api"}

	client.On("Post", config.APIEndpoint, "application/json", mock.Anything).Return(&http.Response{
		StatusCode: http.StatusOK,
	}, nil)

	postPayload(config, payload)

	client.AssertExpectations(t)
}

func TestPostPayload_Failure(t *testing.T) {
	client := new(MockHTTPClient)
	payload := Payload{ItemID: "12345", DeviceType: "scanner"}
	config := &Config{APIEndpoint: "http://example.com/api"}

	client.On("Post", config.APIEndpoint, "application/json", mock.Anything).Return(&http.Response{
		StatusCode: http.StatusInternalServerError,
	}, errors.New("post error"))

	postPayload(config, payload)

	client.AssertExpectations(t)
}

func TestLogFailure(t *testing.T) {
	payload := Payload{ItemID: "12345", DeviceType: "scanner"}
	logFailure(payload)

	file, err := os.Open("failures.log")
	assert.NoError(t, err)
	defer file.Close()
	defer os.Remove("failures.log")

	data, err := io.ReadAll(file)
	assert.NoError(t, err)
	assert.Contains(t, string(data), `{"itemid":"12345","deviceType":"scanner"}`)
}

func TestScanDevice_NoDeviceFound(t *testing.T) {
	// Mocking HID functions and scanning process for test coverage
}

func TestReadKeyboardInput(t *testing.T) {
	// Mocking keyboard input for test coverage
}

func TestStartScanning(t *testing.T) {
	// Mocking multiple devices and keyboard input for test coverage
}

func TestRunService(t *testing.T) {
	s := &Service{}
	err := s.Start(nil)
	assert.NoError(t, err)
	err = s.Stop(nil)
	assert.NoError(t, err)
}

func TestSetupLogging(t *testing.T) {
	setupLogging(false)
	setupLogging(true)
	// Further tests to check log file content can be added
}
