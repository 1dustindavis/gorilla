package service

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOptionalInstallResponseIconPathIsAdditive(t *testing.T) {
	populated, err := json.Marshal(optionalInstallResponseItem{IconPath: `C:\ProgramData\Gorilla\cache\catalog-icons\app.png`})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(populated), `"iconPath":"C:\\ProgramData\\Gorilla\\cache\\catalog-icons\\app.png"`) {
		t.Fatalf("iconPath missing from response JSON: %s", populated)
	}

	empty, err := json.Marshal(optionalInstallResponseItem{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(empty), `"iconPath"`) {
		t.Fatalf("empty iconPath should be omitted: %s", empty)
	}
}
