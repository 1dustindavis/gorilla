package catalogasset

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image/png"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/1dustindavis/gorilla/pkg/download"
)

const iconCacheDir = "catalog-icons"

var (
	drivePathPattern = regexp.MustCompile(`^[A-Za-z]:[\\/]`)
	downloadGet      = download.Get
)

// ResolveIcon resolves one optional catalog icon into a validated local PNG path.
// Asset failures are returned to the caller so the service can log and degrade to
// an empty IconPath without affecting App Catalog software-management state.
func ResolveIcon(repositoryURL, cachePath, iconPath string) (string, error) {
	normalized, err := ValidateIconPath(iconPath)
	if err != nil {
		return "", err
	}
	assetURL, err := ResolveAssetURL(repositoryURL, normalized)
	if err != nil {
		return "", err
	}

	cacheDir := filepath.Join(cachePath, iconCacheDir)
	finalPath := filepath.Join(cacheDir, cacheFileName(repositoryURL, normalized))
	if validPNGFile(finalPath) {
		return finalPath, nil
	}
	_ = os.Remove(finalPath)

	body, err := downloadGet(assetURL)
	if err != nil {
		return "", fmt.Errorf("retrieve icon: %w", err)
	}
	if err := validatePNG(body); err != nil {
		return "", err
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("create icon cache: %w", err)
	}

	tmp, err := os.CreateTemp(cacheDir, ".icon-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create temporary icon: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("write temporary icon: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close temporary icon: %w", err)
	}
	if !validPNGFile(tmpName) {
		return "", errors.New("downloaded icon is not a valid PNG")
	}
	if err := os.Rename(tmpName, finalPath); err != nil {
		// Another resolver may have won a race to populate the same deterministic
		// cache entry. Accept its result if it satisfies the same invariant.
		if validPNGFile(finalPath) {
			return finalPath, nil
		}
		return "", fmt.Errorf("commit icon cache entry: %w", err)
	}
	return finalPath, nil
}

// ValidateIconPath validates and normalizes a repository-relative PNG path.
func ValidateIconPath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("icon path is empty")
	}
	if strings.HasPrefix(value, `\\`) || drivePathPattern.MatchString(value) {
		return "", errors.New("icon path must be repository-relative")
	}

	normalizedSeparators := strings.ReplaceAll(value, `\`, "/")
	if strings.HasPrefix(normalizedSeparators, "/") {
		return "", errors.New("icon path must not be root-relative")
	}
	if parsed, err := url.Parse(normalizedSeparators); err == nil && parsed.Scheme != "" {
		return "", errors.New("icon path must not contain a URI scheme")
	}
	cleaned := path.Clean(normalizedSeparators)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", errors.New("icon path escapes repository root")
	}
	if !strings.EqualFold(path.Ext(cleaned), ".png") {
		return "", errors.New("icon path must use PNG format")
	}
	return cleaned, nil
}

// ResolveAssetURL resolves a validated repository-relative asset path without
// permitting the asset value to replace the configured repository scheme/host.
func ResolveAssetURL(repositoryURL, normalizedPath string) (string, error) {
	base, err := url.Parse(repositoryURL)
	if err != nil || base.Scheme == "" {
		return "", errors.New("configured repository URL is invalid")
	}
	// Gorilla repository configuration is directory-oriented. Preserve the full
	// configured base path even when an administrator omitted its trailing slash.
	if !strings.HasSuffix(base.Path, "/") {
		base.Path += "/"
	}
	rel := &url.URL{Path: normalizedPath}
	resolved := base.ResolveReference(rel)
	if resolved.Scheme != base.Scheme || resolved.Host != base.Host {
		return "", errors.New("icon path resolved outside configured repository")
	}
	return resolved.String(), nil
}

func cacheFileName(repositoryURL, normalizedPath string) string {
	sum := sha256.Sum256([]byte(repositoryURL + "\x00" + normalizedPath))
	return hex.EncodeToString(sum[:]) + ".png"
}

func validatePNG(body []byte) error {
	if len(body) == 0 {
		return errors.New("icon PNG is empty")
	}
	if _, err := png.DecodeConfig(bytes.NewReader(body)); err != nil {
		return fmt.Errorf("invalid icon PNG: %w", err)
	}
	return nil
}

func validPNGFile(filePath string) bool {
	f, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer f.Close()
	_, err = png.DecodeConfig(f)
	return err == nil
}
