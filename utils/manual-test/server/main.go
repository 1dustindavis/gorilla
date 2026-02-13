package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func main() {
	var root string
	var addr string

	flag.StringVar(&root, "root", "build/manual-test/server-root", "path to static content root")
	flag.StringVar(&addr, "addr", ":8080", "listen address")
	flag.Parse()

	absRoot, err := filepath.Abs(root)
	if err != nil {
		log.Fatalf("unable to resolve root path: %v", err)
	}

	info, err := os.Stat(absRoot)
	if err != nil {
		log.Fatalf("unable to access root path %s: %v", absRoot, err)
	}
	if !info.IsDir() {
		log.Fatalf("root path is not a directory: %s", absRoot)
	}

	fileServer := http.FileServer(http.Dir(absRoot))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		fileServer.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})

	fmt.Printf("Serving %s on http://localhost%s\n", absRoot, addr)
	fmt.Println("Expected Gorilla paths:")
	fmt.Printf("  http://localhost%s/manifests/example_manifest.yaml\n", addr)
	fmt.Printf("  http://localhost%s/catalogs/example_catalog.yaml\n", addr)
	fmt.Printf("  http://localhost%s/gorilla.exe\n", addr)

	log.Fatal(http.ListenAndServe(addr, handler))
}
