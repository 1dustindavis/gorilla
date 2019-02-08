package download

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
)

var (
	// Config is a global variable that the allows another package to set the config for `download`
	Config config.Configuration
)

// File downloads a provided url to the file path specified.
// Timeout is 10 seconds
// Will only write to disk if http status code is 2XX
func File(file string, url string) error {
	// Get the absolute file path
	_, fileName := path.Split(url)
	absPath := filepath.Join(file, fileName)

	// Create the dir and file
	err := os.MkdirAll(filepath.Clean(file), 0755)
	if err != nil {
		gorillalog.Warn("Unable to make filepath:", file, err)
	}

	out, err := os.Create(filepath.Clean(absPath))
	if err != nil {
		return err
	}
	defer out.Close()

	// Declare the http client
	var client *http.Client

	// If TLSAuth is true, configure server and client certs
	if Config.TLSAuth {
		// Load	the client certificate and private key
		clientCert, err := tls.LoadX509KeyPair(Config.TLSClientCert, Config.TLSClientKey)
		if err != nil {
			return err
		}

		// Load server certificates
		serverCert, err := ioutil.ReadFile(Config.TLSServerCert)
		if err != nil {
			return err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(serverCert)

		// Setup the tls configuration
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{clientCert},
			RootCAs:      caCertPool,
			// Insecure, but might need to be an option for odd configurations in the future
			// Renegotiation: tls.RenegotiateFreelyAsClient,
		}
		tlsConfig.BuildNameToCertificate()

		// Setup the http client
		client = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
				Dial: (&net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 10 * time.Second,
				}).Dial,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		}
	} else {
		// Setup our http client without tls auth
		// Defining the transport separately so we can add a `file://` protocol
		transport := &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 10 * time.Second,
			}).Dial,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}

		// Register a file handler so `file://` works
		transport.RegisterProtocol("file", http.NewFileTransport(http.Dir("/")))

		// Create the client using our custom transport
		client = &http.Client{Transport: transport}
	}

	// Build the request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		gorillalog.Warn("Unable to request url:", url, err)
	}

	// If we have a user and pass, configure basic auth
	if Config.AuthUser != "" && Config.AuthPass != "" {
		req.SetBasicAuth(Config.AuthUser, Config.AuthPass)
	}

	// Actually send the request, using the client we setup
	// Storing the response in resp
	resp, err := client.Do(req)

	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check that the request was successful
	if resp.StatusCode != 200 {
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
	if shaHash != strings.ToLower(sha) {
		gorillalog.Debug("File hash does not match expected value:", file)
		return false
	}
	return true
}

// IfNeeded takes the same values as Download plus a hash as a string
// It will check if the file already exists, by comparing the hash
// If the hash does not match, it will attempt to download the file
// Once downloaded it will attempt to verify the hash again
func IfNeeded(absFile string, url string, hash string) bool {
	// If the file exists, check the hash
	var verified = false
	if _, err := os.Stat(absFile); err == nil {
		verified = Verify(absFile, hash)
	}

	// If hash failed, download the installer
	if !verified {
		absPath, _ := filepath.Split(absFile)
		gorillalog.Info("Downloading", url, "to", absPath)
		// Download the installer
		err := File(absPath, url)
		if err != nil {
			gorillalog.Warn("Unable to retrieve package:", url, err)
			return verified
		}
		verified = Verify(absFile, hash)
	}

	// return the status of verified
	return verified
}
