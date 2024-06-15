package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/karalabe/hid"
	"github.com/kardianos/service"
	"github.com/sirupsen/logrus"
)

// Config represents the configuration for the application
type Config struct {
	APIEndpoint      string `json:"apiEndpoint"`
	NumberOfScanners int    `json:"numberOfScanners"`
	RescanInterval   int    `json:"rescanInterval"`
	Keyboard         bool   `json:"keyboard"`
}

// Payload represents the data to be sent to the API
type Payload struct {
	ItemID     string `json:"itemid"`
	DeviceType string `json:"deviceType"`
}

func (f *Payload) CleanItemId() {
	result := f.ItemID
	idIndex := strings.Index(result, "id=")
	if idIndex != -1 {
		//Extract the substring after "id="
		f.ItemID = result[idIndex+len("id="):]
	}
}

// Service represents the Windows service
type Service struct {
	wg sync.WaitGroup
}

var logger = logrus.New()

// readConfig reads the configuration from a file
func readConfig() (*Config, error) {
	file, err := os.Open("config.json")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

var httpPost = func(url, contentType string, body io.Reader) (*http.Response, error) {
	return http.Post(url, contentType, body)
}

func postPayload(config *Config, payload Payload) {
	jsonData, err := json.Marshal(payload)
	payload.CleanItemId()
	if err != nil {
		logger.Errorf("Error marshaling payload: %v", err)
		logFailure(payload)
		return
	}

	resp, err := httpPost(config.APIEndpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil || resp.StatusCode != http.StatusOK {
		logger.Errorf("Error posting payload: %v, response code: %v", err, resp.StatusCode)
		logFailure(payload)
		return
	}
	logger.Infof("Successfully posted payload: %v", payload)
}

// logFailure logs the payload to the event log and saves it to a file
func logFailure(payload Payload) {
	logger.Errorf("Failed to post payload: %v", payload)
	file, err := os.OpenFile("failures.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logger.Errorf("Error opening failures.log: %v", err)
		return
	}
	defer file.Close()
	data, err := json.Marshal(payload)
	if err != nil {
		logger.Errorf("Error marshaling payload: %v", err)
		return
	}
	_, err = file.WriteString(fmt.Sprintf("%s\n", data))
	if err != nil {
		logger.Errorf("Error writing to failures.log: %v", err)
	}
}

// scanDevice reads the data from a HID device and sends the payload to the channel
func scanDevice(config *Config, deviceID int, payloadCh chan Payload) {
	for {
		devices := hid.Enumerate(0, 0)
		if deviceID >= len(devices)) {
			logger.Warnf("No device found for deviceID %d. Rescanning in %d seconds...", deviceID, config.RescanInterval)
			time.Sleep(time.Duration(config.RescanInterval) * time.Second)
			continue
		}

		device, err := devices[deviceID].Open()
		if err != nil {
			logger.Errorf("Error opening device: %v", err)
			time.Sleep(time.Duration(config.RescanInterval) * time.Second)
			continue
		}
		defer device.Close()

		buf := make([]byte, 256)
		for {
			n, err := device.Read(buf)
			if err != nil {
				logger.Errorf("Error reading from device: %v", err)
				break
			}

			if n > 0 {
				// Convert byte buffer to string
				payload := Payload{
					ItemID:     string(buf[:n]),
					DeviceType: fmt.Sprintf("scanner%d", deviceID),
				}
				payloadCh <- payload
			}
		}
	}
}

// readKeyboardInput reads keyboard input and sends the payload to the channel
func readKeyboardInput(payloadCh chan Payload) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		//Extract the substring after "id="
		payload := Payload{
			ItemID:     scanner.Text(),
			DeviceType: "keyboard",
		}
		payloadCh <- payload
	}
	if err := scanner.Err(); err != nil {
		logger.Fatalf("Error reading standard input: %v", err)
	}
}

// startScanning starts scanning from multiple devices
func startScanning(config *Config, payloadCh chan Payload) {
	for i := 0; i < config.NumberOfScanners; i++ {
		go scanDevice(config, i, payloadCh)
	}
	if config.Keyboard {
		go readKeyboardInput(payloadCh)
	}
}

// runService runs the service
func (s *Service) runService() {
	config, err := readConfig()
	if err != nil {
		logger.Fatalf("Error reading config: %v", err)
	}
	payloadCh := make(chan Payload)
	go startScanning(config, payloadCh)
	for payload := range payloadCh {
		go postPayload(config, payload)
	}
}

// Start implements the Start method of the service
func (s *Service) Start(svc service.Service) error {
	s.wg.Add(1)
	go s.runService()
	return nil
}

// Stop implements the Stop method of the service
func (s *Service) Stop(svc service.Service) error {
	s.wg.Done()
	return nil
}

// setupLogging configures logging to a file and optionally to stdout
func setupLogging(serviceMode bool) {
	logFile, err := os.OpenFile("service.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Error opening log file: %v", err)
	}

	jsonFormatter := &logrus.JSONFormatter{}
	textFormatter := &logrus.TextFormatter{
		FullTimestamp: true,
	}

	fileLogger := logrus.New()
	fileLogger.SetOutput(logFile)
	fileLogger.SetFormatter(jsonFormatter)

	consoleLogger := logrus.New()
	consoleLogger.SetOutput(os.Stdout)
	consoleLogger.SetFormatter(textFormatter)

	logger.SetOutput(io.MultiWriter(logFile, os.Stdout))
	logger.SetFormatter(jsonFormatter)

	if serviceMode {
		logger.SetLevel(logrus.InfoLevel)
		fileLogger.SetLevel(logrus.InfoLevel)
		consoleLogger.SetLevel(logrus.InfoLevel)
	} else {
		logger.SetLevel(logrus.DebugLevel)
		fileLogger.SetLevel(logrus.DebugLevel)
		consoleLogger.SetLevel(logrus.DebugLevel)
	}
}

func main() {
	setupLogging(true)

	svcConfig := &service.Config{
		Name:        "SPCBarcodeService",
		DisplayName: "SPC Barcode Service",
		Description: "Service for reading HID scanner output and posting to an API",
	}

	svc := &Service{}
	s, err := service.New(svc, svcConfig)
	if err != nil {
		logger.Fatalf("Error creating service: %v", err)
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "install":
			err = s.Install()
			if err != nil {
				logger.Fatalf("Error installing service: %v", err)
			}
			fmt.Println("Service installed successfully.")
			return
		case "uninstall":
			err = s.Uninstall()
			if err != nil {
				logger.Fatalf("Error uninstalling service: %v", err)
			}
			fmt.Println("Service uninstalled successfully.")
			return
		case "interactive":
			svc.runService()
			return
		}
	}

	err = s.Run()
	if err != nil {
		logger.Fatalf("Error running service: %v", err)
	}
}