package installer

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/catalog"
)

func TestInstallScriptsReturnProcessStartError(t *testing.T) {
	orig := execCommand
	execCommand = func(string, ...string) *exec.Cmd {
		return exec.Command(filepath.Join(t.TempDir(), "missing-powershell.exe"))
	}
	defer func() { execCommand = orig }()

	tests := []struct {
		name   string
		run    func(catalog.Item, string) (bool, error)
		script func(*catalog.Item)
	}{
		{
			name: "preinstall",
			run:  preinstallScript,
			script: func(item *catalog.Item) {
				item.PreScript = "Write-Output pre"
			},
		},
		{
			name: "postinstall",
			run:  postinstallScript,
			script: func(item *catalog.Item) {
				item.PostScript = "Write-Output post"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := catalog.Item{}
			tt.script(&item)
			success, err := tt.run(item, t.TempDir())
			if err == nil {
				t.Fatal("expected process start error")
			}
			if success {
				t.Fatal("expected failed script execution")
			}
		})
	}
}
