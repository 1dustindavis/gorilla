package status

import (
	"testing"

	"github.com/1dustindavis/gorilla/pkg/catalog"
)

// The future App Catalog observation adapter must preserve this authority.
// Exercise the shared check directly, without a second version comparator.
func TestCheckStatusRegistryVersionAuthority(t *testing.T) {
	previous := RegistryItems
	t.Cleanup(func() { RegistryItems = previous })
	RegistryItems = map[string]RegistryApplication{
		"example": {Name: "Example", Version: "1.7"},
	}

	for _, tc := range []struct {
		name           string
		catalogVersion string
		checkVersion   string
		wantAction     bool
	}{
		{"catalog newer but check satisfied", "2.0", "1.5", false},
		{"catalog older but check unsatisfied", "1.5", "2.0", true},
		{"check exactly satisfied", "2.0", "1.7", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := catalog.Item{
				DisplayName: "Example",
				Version:     tc.catalogVersion,
				Check: catalog.InstallCheck{
					Registry: catalog.RegCheck{Name: "Example", Version: tc.checkVersion},
				},
			}
			for _, action := range []string{"install", "update"} {
				got, err := CheckStatus(item, action, "")
				if err != nil {
					t.Fatal(err)
				}
				if got != tc.wantAction {
					t.Fatalf("%s: actionNeeded=%v, want %v (catalog=%s check=%s installed=1.7)",
						action, got, tc.wantAction, tc.catalogVersion, tc.checkVersion)
				}
			}
		})
	}
}
