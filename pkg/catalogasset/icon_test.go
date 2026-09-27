package catalogasset

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestValidateIconPath(t *testing.T) {
	for _, value := range []string{
		"icons/app.png",
		"assets/app.png",
		"assets/icons/app.png",
		"a/b/c/app.png",
		`assets\icons\app.png`,
		"icons/app.PNG",
	} {
		t.Run("accept_"+strings.NewReplacer("/", "_", `\`, "_").Replace(value), func(t *testing.T) {
			got, err := ValidateIconPath(value)
			if err != nil {
				t.Fatalf("ValidateIconPath(%q): %v", value, err)
			}
			if strings.Contains(got, `\`) {
				t.Fatalf("normalized path %q still contains backslash", got)
			}
		})
	}

	for _, value := range []string{
		"",
		"../app.png",
		"a/../../app.png",
		`a\..\..\app.png`,
		"/app.png",
		`C:\app.png`,
		"C:/app.png",
		`\\server\share\app.png`,
		"https://example.com/app.png",
		"http://example.com/app.png",
		"file:///tmp/app.png",
		"icons/app.jpg",
		"icons/app.svg",
	} {
		t.Run("reject_"+strings.NewReplacer("/", "_", `\`, "_").Replace(value), func(t *testing.T) {
			if _, err := ValidateIconPath(value); err == nil {
				t.Fatalf("ValidateIconPath(%q) unexpectedly succeeded", value)
			}
		})
	}
}

func TestResolveAssetURLPreservesRepositoryBase(t *testing.T) {
	for _, tc := range []struct {
		base string
		want string
	}{
		{"https://repo.example/gorilla/", "https://repo.example/gorilla/icons/app.png"},
		{"https://repo.example/gorilla", "https://repo.example/gorilla/icons/app.png"},
		{"file:///tmp/gorilla/", "file:///tmp/gorilla/icons/app.png"},
	} {
		got, err := ResolveAssetURL(tc.base, "icons/app.png")
		if err != nil {
			t.Fatalf("ResolveAssetURL(%q): %v", tc.base, err)
		}
		if got != tc.want {
			t.Fatalf("ResolveAssetURL(%q) = %q, want %q", tc.base, got, tc.want)
		}
	}
}

func TestResolveIconCachesValidatedPNG(t *testing.T) {
	original := downloadGet
	t.Cleanup(func() { downloadGet = original })
	calls := 0
	downloadGet = func(url string) ([]byte, error) {
		calls++
		if url != "https://repo.example/gorilla/icons/app.png" {
			t.Fatalf("unexpected URL %q", url)
		}
		return testPNG(t), nil
	}
	cache := t.TempDir()

	first, err := ResolveIcon("https://repo.example/gorilla/", cache, "icons/app.png")
	if err != nil {
		t.Fatal(err)
	}
	second, err := ResolveIcon("https://repo.example/gorilla/", cache, "icons/app.png")
	if err != nil {
		t.Fatal(err)
	}
	if first != second || calls != 1 {
		t.Fatalf("cache miss: first=%q second=%q calls=%d", first, second, calls)
	}
	if filepath.Dir(first) != filepath.Join(cache, iconCacheDir) {
		t.Fatalf("cache path %q not under icon cache", first)
	}
	if !validPNGFile(first) {
		t.Fatalf("cached icon is not valid PNG: %q", first)
	}
}

func TestResolveIconCacheKeySeparatesPathsAndRepositories(t *testing.T) {
	original := downloadGet
	t.Cleanup(func() { downloadGet = original })
	downloadGet = func(string) ([]byte, error) { return testPNG(t), nil }
	cache := t.TempDir()

	a, err := ResolveIcon("https://repo-a.example/gorilla/", cache, "icons/app.png")
	if err != nil {
		t.Fatal(err)
	}
	b, err := ResolveIcon("https://repo-a.example/gorilla/", cache, "branding/app.png")
	if err != nil {
		t.Fatal(err)
	}
	c, err := ResolveIcon("https://repo-b.example/gorilla/", cache, "icons/app.png")
	if err != nil {
		t.Fatal(err)
	}
	if a == b || a == c || b == c {
		t.Fatalf("cache key collision: %q %q %q", a, b, c)
	}
}

func TestResolveIconFailuresDoNotExposePartialAsset(t *testing.T) {
	original := downloadGet
	t.Cleanup(func() { downloadGet = original })
	cache := t.TempDir()

	downloadGet = func(string) ([]byte, error) { return nil, errors.New("offline") }
	if got, err := ResolveIcon("https://repo.example/", cache, "icons/app.png"); err == nil || got != "" {
		t.Fatalf("download failure returned path=%q err=%v", got, err)
	}

	downloadGet = func(string) ([]byte, error) { return []byte("not a png"), nil }
	if got, err := ResolveIcon("https://repo.example/", cache, "icons/app.png"); err == nil || got != "" {
		t.Fatalf("malformed PNG returned path=%q err=%v", got, err)
	}
	entries, err := os.ReadDir(filepath.Join(cache, iconCacheDir))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("failed resolutions left cache files: %v", entries)
	}
}

func TestResolveIconReplacesCorruptCacheEntry(t *testing.T) {
	original := downloadGet
	t.Cleanup(func() { downloadGet = original })
	calls := 0
	downloadGet = func(string) ([]byte, error) {
		calls++
		return testPNG(t), nil
	}
	cache := t.TempDir()
	normalized, _ := ValidateIconPath("icons/app.png")
	cacheDir := filepath.Join(cache, iconCacheDir)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	final := filepath.Join(cacheDir, cacheFileName("https://repo.example/", normalized))
	if err := os.WriteFile(final, []byte("truncated"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveIcon("https://repo.example/", cache, "icons/app.png")
	if err != nil {
		t.Fatal(err)
	}
	if got != final || calls != 1 || !validPNGFile(final) {
		t.Fatalf("corrupt cache was not refreshed: got=%q calls=%d", got, calls)
	}
}

func TestResolveIconCacheDirectoryFailure(t *testing.T) {
	original := downloadGet
	t.Cleanup(func() { downloadGet = original })
	downloadGet = func(string) ([]byte, error) { return testPNG(t), nil }
	root := t.TempDir()
	cacheFile := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(cacheFile, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if got, err := ResolveIcon("https://repo.example/", cacheFile, "icons/app.png"); err == nil || got != "" {
		t.Fatalf("cache directory failure returned path=%q err=%v", got, err)
	}
}
