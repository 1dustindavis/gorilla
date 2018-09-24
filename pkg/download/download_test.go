package download

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/1dustindavis/gorilla/pkg/config"
)

var (
	testFile    = "testdata/hashtest.txt"
	validHash   = "dca48f4e34541c52d12351479454b3af6d87d8dc23ec48f68962f062d8703de3"
	invalidHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

// TestVerify tests hash comparison
func TestVerify(t *testing.T) {
	// Run the code
	validTest := Verify(testFile, validHash)
	invalidTest := Verify(testFile, invalidHash)

	// Compare output
	if have, want := validTest, true; have != want {
		t.Errorf("have %v, want %v", have, want)
	}

	// Compare output
	if have, want := invalidTest, false; have != want {
		t.Errorf("have %v, want %v", have, want)
	}
}

// serveTestFile writes the contents of `testFile` to the http repsonse
func serveTestFile(w http.ResponseWriter, r *http.Request) {
	// Open are test file
	testSource, err := os.Open(testFile)
	if err != nil {
		log.Fatal(err)
	}

	// Copy the contents to the http response
	_, err = io.Copy(w, testSource)
	if err != nil {
		log.Fatal(err)
	}
}

func serveTimeout(w http.ResponseWriter, r *http.Request) {
	// Sleep 11 seconds to simiulate a timeout
	time.Sleep(11 * time.Second)
	serveTestFile(w, r)
}

func serve404(w http.ResponseWriter, r *http.Request) {
	// Write a 404 response header
	w.WriteHeader(http.StatusNotFound)
}

func serveBasicAuth(w http.ResponseWriter, r *http.Request) {
	// Parse the username and password
	user, pass, _ := r.BasicAuth()

	// If correct, return 200, else 401
	if user == "frank" && pass == "beans" {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusUnauthorized)
	}
}

func serveTLSAuth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// route directs http requests to the correct function
func router() *http.ServeMux {
	h := http.NewServeMux()
	h.HandleFunc("/hashtest.txt", serveTestFile)
	h.HandleFunc("/timeout", serveTimeout)
	h.HandleFunc("/404", serve404)
	h.HandleFunc("/basicauth", serveBasicAuth)
	h.HandleFunc("/tlsauth", serveTLSAuth)
	return h
}

// TestFileHash verifies that a file is downloaded properly
func TestFileHash(t *testing.T) {
	// Create a temporary directory
	dir, err := ioutil.TempDir("", "gorilla_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create a test server
	ts := httptest.NewServer(router())
	defer ts.Close()

	// Run the code
	File(dir, ts.URL+"/hashtest.txt")

	// Validate the hash to confirm it was downloaded properly
	if !Verify(filepath.Join(dir, "hashtest.txt"), validHash) {
		t.Errorf("Hash does not match downloaded test file!")
	}

}

// TestFileTimeout verifies a connection will timeout
func TestFileTimeout(t *testing.T) {
	// Check it this is short run
	if testing.Short() {
		t.Skip("skipping TestFileTimeout in short mode.")
	}

	fmt.Println("Run with '-short' to skip this longer test")

	// Create a temporary directory
	dir, err := ioutil.TempDir("", "gorilla_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create a test server
	ts := httptest.NewServer(router())
	defer ts.Close()

	// Run the code
	fileErr := File(dir, ts.URL+"/timeout")

	// Check the error output to confirm we timedout
	if fileErr != nil {
		if !strings.Contains(fileErr.Error(), "timeout") {
			t.Errorf("Error received from File() did not include 'timeout':\n%v", fileErr)
		}
	} else {
		t.Errorf("File() did not return an error when running 'TestFileTimeout'")
	}

}

// TestFileStatus verifies status codes are respected
func TestFileStatus(t *testing.T) {
	// Create a temporary directory
	dir, err := ioutil.TempDir("", "gorilla_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create a test server
	ts := httptest.NewServer(router())
	defer ts.Close()

	// Run the code
	fileErr := File(dir, ts.URL+"/404")

	// Check the error output to confirm we received a 404
	if fileErr != nil {
		if !strings.Contains(fileErr.Error(), "404") {
			t.Errorf("Error received from File() did not include '404':\n%v", fileErr)
		}
	} else {
		t.Errorf("File() did not return an error when returning a 404")
	}

}

// TestFileBasicAuth verifies username and password are included in headers
func TestFileBasicAuth(t *testing.T) {
	// Create a temporary directory
	dir, err := ioutil.TempDir("", "gorilla_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create a test server
	ts := httptest.NewServer(router())
	defer ts.Close()

	// Save the orginal config
	origConfig := config.Current
	defer func() { config.Current = origConfig }()

	// Setup basic auth
	config.Current.AuthUser = "frank"
	config.Current.AuthPass = "beans"

	// Run the code
	fileErr := File(dir, ts.URL+"/basicauth")

	// Check that we did not receive an error
	if fileErr != nil {
		t.Errorf("File download with basic auth failed':\n%v", fileErr)
	}

}

// TestFileTLS verifies TLS auth is functioning
func TestFileTLS(t *testing.T) {
	// Create a temporary directory
	dir, err := ioutil.TempDir("", "gorilla_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Save the orginal config
	origConfig := config.Current
	defer func() { config.Current = origConfig }()

	// Setup TLS auth
	config.Current.TLSAuth = true
	config.Current.TLSClientCert = "testdata/client.pem"
	config.Current.TLSClientKey = "testdata/client.key"
	config.Current.TLSServerCert = "testdata/server.pem"
	serverKeyPath := "testdata/server.key"

	// Create a test server
	ts := httptest.NewUnstartedServer(router())

	// Prepare ca certs
	serverCert, _ := ioutil.ReadFile(config.Current.TLSServerCert)
	serverKey, _ := ioutil.ReadFile(serverKeyPath)

	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(serverCert)

	cert, err := tls.X509KeyPair(serverCert, serverKey)
	if err != nil {
		log.Fatal(err)
	}

	// Set TLS configuration
	ts.TLS = &tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
		Certificates: []tls.Certificate{cert},
	}
	ts.TLS.BuildNameToCertificate()

	// Start server
	ts.StartTLS()
	defer ts.Close()

	// Compile the url with hostname instead of ip address
	u, err := url.Parse(ts.URL)
	if err != nil {
		log.Fatal(err)
	}
	tlsURL := "https://localhost:" + u.Port() + "/tlsauth"

	// Run the code
	fileErr := File(dir, tlsURL)

	// Check that we did not receive an error
	if fileErr != nil {
		t.Errorf("File download with TLS auth failed':\n%v", fileErr)
	}

}
