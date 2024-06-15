package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/karalabe/hid"
	"github.com/kardianos/service"
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
		log.Printf("Error marshaling payload: %v\n", err)
		logFailure(payload)
		return
	}

	resp, err := httpPost(config.APIEndpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Printf("Error posting payload: %v, response code: %v\n", err, resp.StatusCode)
		logFailure(payload)
		return
	}
	log.Printf("Successfully posted payload: %v\n", payload)
}

// logFailure logs the payload to the event log and saves it to a file
func logFailure(payload Payload) {
	log.Printf("Failed to post payload: %v\n", payload)
	file, err := os.OpenFile("failures.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Error opening failures.log: %v\n", err)
		return
	}
	defer file.Close()
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling payload: %v\n", err)
		return
	}
	_, err = file.WriteString(fmt.Sprintf("%s\n", data))
	if err != nil {
		log.Printf("Error writing to failures.log: %v\n", err)
	}
}

// scanDevice reads the data from a HID device and sends the payload to the channel
func scanDevice(config *Config, deviceID int, payloadCh chan Payload) {
	for {
		devices := hid.Enumerate(0, 0)
		if deviceID >= len(devices) {
			log.Printf("No device found for deviceID %d. Rescanning in %d seconds...\n", deviceID, config.RescanInterval)
			time.Sleep(time.Duration(config.RescanInterval) * time.Second)
			continue
		}

		device, err := devices[deviceID].Open()
		if err != nil {
			log.Printf("Error opening device: %v\n", err)
			time.Sleep(time.Duration(config.RescanInterval) * time.Second)
			continue
		}
		defer device.Close()

		buf := make([]byte, 256)
		for {
			n, err := device.Read(buf)
			if err != nil {
				log.Printf("Error reading from device: %v\n", err)
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
		log.Fatalf("Error reading standard input: %v\n", err)
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
		log.Fatalf("Error reading config: %v\n", err)
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

	if serviceMode {
		log.SetOutput(logFile)
	} else {
		mw := io.MultiWriter(os.Stdout, logFile)
		log.SetOutput(mw)
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
		log.Fatalf("Error creating service: %v\n", err)
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "install":
			err = s.Install()
			if err != nil {
				log.Fatalf("Error installing service: %v\n", err)
			}
			fmt.Println("Service installed successfully.")
			return
		case "uninstall":
			err = s.Uninstall()
			if err != nil {
				log.Fatalf("Error uninstalling service: %v\n", err)
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
		log.Fatalf("Error running service: %v\n", err)
	}
}
