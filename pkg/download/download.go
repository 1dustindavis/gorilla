package download

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
)

// File downloads a provided url to the file path specified.
// Timeout is 10 seconds
// Will only write to disk if http status code is 2XX
func File(file string, url string) error {
	// Get the absolute file path
	tokens := strings.Split(url, "/")
	fileName := tokens[len(tokens)-1]
	absPath := filepath.Join(file, fileName)

	// Create the dir and file
	err := os.MkdirAll(filepath.Clean(file), 0755)
	out, err := os.Create(filepath.Clean(absPath))
	if err != nil {
		return err
	}
	defer out.Close()

	// Setup out http client
	client := &http.Client{
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 10 * time.Second,
			}).Dial,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	// Build the request
	req, err := http.NewRequest("GET", url, nil)

	// If configured, set our auth headers
	if config.AuthUser != "" && config.AuthPass != "" {
		req.SetBasicAuth(config.AuthUser, config.AuthPass)
	}

	// Actually send the request, using the client we setup
	// Storing the response in resp
	resp, err := client.Do(req)

	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check that the request was successful
	if resp.StatusCode <= 200 && resp.StatusCode >= 299 {
		return fmt.Errorf("%s : Download status code: %d", fileName, resp.StatusCode)
	}

	// Write the body of the response to disk
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

// Verify compares a provided hash to the actual hash of a file
func Verify(file string, sha string) bool {
	f, err := os.Open(file)
	if err != nil {
		gorillalog.Warn("Unable to open file:", err)
		return false
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		gorillalog.Warn("Unable to verify hash due to IO error:", err)
		return false
	}
	shaHash := hex.EncodeToString(h.Sum(nil))
	if shaHash != sha {
		gorillalog.Warn("Downloaded file hash does not match catalog hash")
		return false
	}
	return true
}
