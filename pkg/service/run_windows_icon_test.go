//go:build windows

package service

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/appcatalog"
)

func TestListOptionalInstallsResponseEmitsResolvedIconPathOnly(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "icon-response-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	contract := appcatalog.Item{
		ItemName:    "Example",
		DisplayName: "Example App",
		IconPath:    `C:\ProgramData\Gorilla\cache\catalog-icons\resolved.png`,
		Observation: appcatalog.Observation{State: appcatalog.Absent},
		Policy:      appcatalog.Policy{Optional: true},
	}
	sr := &serviceRunner{}
	req := serviceEnvelope[json.RawMessage]{RequestID: "req-icon", Operation: actionListOptionalInstalls}
	resp := CommandResponse{OptionalItems: []optionalItemDetails{{
		Contract:   contract,
		iconSource: "icons/source.png",
	}}}
	if err := sr.writeSuccessEnvelope(file, req, Command{Action: actionListOptionalInstalls}, resp); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "icons/source.png") {
		t.Fatalf("raw repository icon source leaked into live response: %s", body)
	}

	var envelope serviceEnvelope[listOptionalInstallsResponse]
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Payload.Items) != 1 {
		t.Fatalf("got %d response items", len(envelope.Payload.Items))
	}
	if got := envelope.Payload.Items[0].IconPath; got != contract.IconPath {
		t.Fatalf("iconPath = %q, want %q", got, contract.IconPath)
	}
}
