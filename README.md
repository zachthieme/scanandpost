## Barcode Scanner Service Documentation

### Overview

The HID Scanner Service is a Go application that reads output from HID barcode scanners and posts the scanned data to a specified API endpoint. It supports multiple scanners running in parallel and can be run as a Windows service or in interactive mode. If a post fails, the payload is logged and saved to a file for replay.

### Configuration

The application uses a `config.json` file to configure the API endpoint and the number of scanners.

Create a `config.json` file with the following structure:

```json
{
  "apiEndpoint": "http://example.com/api",
  "numberOfScanners": 2,
  "rescanInterval": 10
}
```

### Installation and Usage

#### Prerequisites

- Go environment setup
- The following Go packages:
  ```sh
  go get github.com/kardianos/service
  go get github.com/karalabe/hid
  ```

#### Build the Executable

1. Clone the repository or download the source code.
2. Navigate to the directory containing `main.go`.
3. Build the executable:
   ```sh
   go build -o SPCBarcodeService main.go
   ```

#### Running the Application

You can run the application in interactive mode or install it as a Windows service.

##### Interactive Mode

To run the application in interactive mode, simply execute the built binary:

```sh
SPCBarcodeService
```

##### Windows Service Mode

To install, start, stop, and uninstall the application as a Windows service, use the following commands:

1. **Install the service**:

   ```sh
   SPCBarcodeService install
   ```

2. **Start the service**:

   ```sh
   net start SPCBarcodeService
   ```

3. **Stop the service**:

   ```sh
   net stop SPCBarcodeService
   ```

4. **Uninstall the service**:
   ```sh
   SPCBarcodeService uninstall
   ```

### Logging and Error Handling

- **Failed Post Requests**: If a post request fails, the payload is logged to the event log and saved to `failures.log` for replay.

### Code Structure

- **Config**: Reads the configuration from `config.json`.
- **Payload**: Defines the structure of the data to be posted.
- **Service**: Implements the Windows service interface using `github.com/kardianos/service`.
- **HID Device Handling**: Uses `github.com/karalabe/hid` to interface with HID devices and read data.
- **Parallel Scanning**: Scans from multiple devices in parallel using Go routines.
- **Payload Posting**: Posts the payload to the configured API endpoint and handles failures.

### Example

Assuming you have two HID barcode scanners connected, and you want to post scanned data to `http://example.com/api`:

1. Create a `config.json` file with the following content:

   ```json
   {
       "apiEndpoint": "http://example.com/api",
       "numberOfScanners": 2,
       "rescanInterval": 10
   }
   ```

2. Build the executable:

   ```sh
   go build SPCBarcodeService.go
   ```

3. Run the application in interactive mode:
   ```sh
   SPCBarcodeService interactive
   ```

### Open Issues

- Service fails to start - not sure why it fails as it dies before logging starts up
