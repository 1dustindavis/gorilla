package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/catalog"
)

func TestInstallResultPreservesScriptFailureDetail(t *testing.T) {
	previousStatus := statusCheckStatus
	previousInstall := installItemFunc
	t.Cleanup(func() {
		statusCheckStatus = previousStatus
		installItemFunc = previousInstall
	})

	statusCheckStatus = func(catalog.Item, string, string) (bool, error) { return true, nil }
	installItemFunc = func(catalog.Item, string, string) (string, error) { return "", nil }

	cachePath := filepath.Join(t.TempDir(), "cache-file")
	if err := os.WriteFile(cachePath, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}

	baseItem := catalog.Item{
		DisplayName: "Scripted App",
		Installer:   catalog.InstallerItem{Type: "msi", Location: "app.msi"},
	}

	preItem := baseItem
	preItem.PreScript = "Write-Output pre"
	preResult := InstallResult(preItem, "install", "", cachePath, false)
	if preResult.ErrorCode != "preinstall_script_failed" || !strings.HasPrefix(preResult.Message, "PreInstall-Script error: ") {
		t.Fatalf("preinstall failure lost diagnostic detail: %+v", preResult)
	}

	postItem := baseItem
	postItem.PostScript = "Write-Output post"
	postResult := InstallResult(postItem, "install", "", cachePath, false)
	if postResult.ErrorCode != "postinstall_script_failed" || !strings.HasPrefix(postResult.Message, "PostInstall-Script error: ") {
		t.Fatalf("postinstall failure lost diagnostic detail: %+v", postResult)
	}
}
