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
)

var (
	testFile       = "testdata/hashtest.txt"
	validHash      = "dca48f4e34541c52d12351479454b3af6d87d8dc23ec48f68962f062d8703de3"
	validHashUpper = "DCA48F4E34541C52D12351479454B3AF6D87D8DC23EC48F68962F062D8703DE3"
	invalidHash    = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

// TestVerify tests hash comparison
func TestVerify(t *testing.T) {
	// Run the code
	validTest := Verify(testFile, validHash)
	validUpperTest := Verify(testFile, validHashUpper)
	invalidTest := Verify(testFile, invalidHash)

	// Compare output
	if have, want := validTest, true; have != want {
		t.Errorf("Valid hash test: have %v, want %v", have, want)
	}

	if have, want := validUpperTest, true; have != want {
		t.Errorf("Valid upper hash test: have %v, want %v", have, want)
	}

	// Compare output
	if have, want := invalidTest, false; have != want {
		t.Errorf("Invalid hash test: have %v, want %v", have, want)
	}
}

// serveTestFile writes the contents of `testFile` to the http response
func serveTestFile(w http.ResponseWriter, r *http.Request) {
	// Open our test file
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

// TestFileHashLocal verifies that a *local* file is downloaded properly
func TestFileHashLocal(t *testing.T) {
	// Create a temporary directory
	dir, err := ioutil.TempDir("", "gorilla_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Get the absolute path of our test file
	testPath, err := filepath.Abs("testdata/hashtest.txt")
	if err != nil {
		t.Fatal(err)
	}
	// Convert the path seperators to slashes
	testPath = filepath.ToSlash(testPath)

	// Run the code
	fmt.Println("Downloading from local path:", testPath)
	File(dir, "file://"+testPath)

	// Validate the hash to confirm it was downloaded properly
	if !Verify(filepath.Join(dir, "hashtest.txt"), validHash) {
		t.Errorf("Hash does not match downloaded test file from a local url!")
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

	// Setup basic auth
	downloadCfg.AuthUser = "frank"
	downloadCfg.AuthPass = "beans"

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

	// Setup TLS auth
	downloadCfg.TLSAuth = true
	downloadCfg.TLSClientCert = "testdata/client.pem"
	downloadCfg.TLSClientKey = "testdata/client.key"
	downloadCfg.TLSServerCert = "testdata/server.pem"
	serverKeyPath := "testdata/server.key"

	// Create a test server
	ts := httptest.NewUnstartedServer(router())

	// Prepare ca certs
	serverCert, _ := ioutil.ReadFile(downloadCfg.TLSServerCert)
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

func copy(src, dst string) error {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("%s could not be opened: %s", src, err)
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("%s could not be created: %s", dst, err)
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

// TestIfNeededValid confirms that a file is not downloaded when a valid copy exists
func TestIfNeededValid(t *testing.T) {

	// Create a temporary directory
	dir, err := ioutil.TempDir("", "gorilla_test")
	if err != nil {
		t.Fatal(err)
	}
	// defer os.RemoveAll(dir)

	// Copy the test file to our temp directory
	tempFile := filepath.Join(dir, "/hashtest.txt")
	err = copy(testFile, tempFile)
	if err != nil {
		t.Error("copy failed: ", err)
	}

	fmt.Println("timestamps")
	// Set the timestamps on our test file, so we can see if it was updated
	testTime := time.Now().Add(-240 * time.Hour) // 10 days
	err = os.Chtimes(tempFile, testTime, testTime)
	if err != nil {
		t.Error(err)
	}

	// Create a test server
	ts := httptest.NewServer(router())
	defer ts.Close()

	// Run the function with our test data and a validHash
	valid := IfNeeded(tempFile, ts.URL+"/hashtest.txt", validHash)
	if !valid {
		t.Error("Unable to download valid file: ", ts.URL+"/hashtest.txt")
	}

	// Get the test file's modification time to see if it was redownloaded
	fileInfo, err := os.Stat(tempFile)
	if err != nil {
		t.Error(err)
	}
	modTime := fileInfo.ModTime()

	if !modTime.Equal(testTime) {
		t.Error("IfNeeded() downloaded a file that *was not* needed!")
	}

}

// TestIfNeededInvalid confirms that a file *is* downloaded when an invalid copy exists
func TestIfNeededInvalid(t *testing.T) {

	// Create a temporary directory
	dir, err := ioutil.TempDir("", "gorilla_test")
	if err != nil {
		t.Fatal(err)
	}
	// defer os.RemoveAll(dir)

	// Copy a file with a different hash to our temp directory
	tempFile := filepath.Join(dir, "/hashtest.txt")
	err = copy("testdata/client.pem", tempFile)
	if err != nil {
		t.Error("copy failed: ", err)
	}

	// Set the timestamps on our test file, so we can see if it was updated
	testTime := time.Now().Add(-240 * time.Hour) // 10 days
	err = os.Chtimes(tempFile, testTime, testTime)
	if err != nil {
		t.Error(err)
	}

	// Create a test server
	ts := httptest.NewServer(router())
	defer ts.Close()

	// Run the function with our test data and a validHash
	valid := IfNeeded(tempFile, ts.URL+"/hashtest.txt", validHash)
	if !valid {
		t.Error("Unable to download valid file: ", ts.URL+"/hashtest.txt")
	}

	// Get the test file's modification time to see if it was redownloaded
	fileInfo, err := os.Stat(tempFile)
	if err != nil {
		t.Error(err)
	}
	modTime := fileInfo.ModTime()

	if modTime.Equal(testTime) {
		t.Error("IfNeeded() did *not* download a file when it *was* needed!")
	}

}
