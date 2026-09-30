package service

import (
	"encoding/json"
	"testing"
)

func TestOptionalInstallResponseDescriptionIsAdditive(t *testing.T) {
	populated, err := json.Marshal(optionalInstallResponseItem{Description: "An example application."})
	if err != nil {
		t.Fatal(err)
	}
	var populatedObject map[string]any
	if err := json.Unmarshal(populated, &populatedObject); err != nil {
		t.Fatal(err)
	}
	if got := populatedObject["description"]; got != "An example application." {
		t.Fatalf("description = %#v, want populated camelCase field", got)
	}

	empty, err := json.Marshal(optionalInstallResponseItem{})
	if err != nil {
		t.Fatal(err)
	}
	var emptyObject map[string]any
	if err := json.Unmarshal(empty, &emptyObject); err != nil {
		t.Fatal(err)
	}
	if _, found := emptyObject["description"]; found {
		t.Fatalf("empty description should be omitted: %s", empty)
	}
}

func TestNewListRequestRemainsCompatibleWithOldDecoder(t *testing.T) {
	type oldListOptionalInstallsRequest struct{}

	var decoded oldListOptionalInstallsRequest
	if err := json.Unmarshal([]byte(`{"refresh":true}`), &decoded); err != nil {
		t.Fatalf("old decoder rejected additive refresh property: %v", err)
	}
}

func TestNewListResponseDecoderAcceptsLegacyResponse(t *testing.T) {
	var decoded listOptionalInstallsResponse
	if err := json.Unmarshal([]byte(`{"items":[]}`), &decoded); err != nil {
		t.Fatalf("new decoder rejected legacy response: %v", err)
	}
	if decoded.SnapshotAvailable != nil {
		t.Fatalf("legacy response snapshotAvailable = %v, want nil", decoded.SnapshotAvailable)
	}
	if decoded.RefreshState != "" || decoded.SnapshotGeneratedAtUTC != "" {
		t.Fatalf("legacy response unexpectedly populated snapshot metadata: %+v", decoded)
	}
}

func TestListOptionalInstallsRequestPreservesRefreshTriState(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		wantNil bool
		want    bool
	}{
		{name: "legacy", payload: `{}`, wantNil: true},
		{name: "snapshot read", payload: `{"refresh":false}`, want: false},
		{name: "snapshot refresh", payload: `{"refresh":true}`, want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var decoded listOptionalInstallsRequest
			if err := json.Unmarshal([]byte(tc.payload), &decoded); err != nil {
				t.Fatal(err)
			}
			if tc.wantNil {
				if decoded.Refresh != nil {
					t.Fatalf("Refresh = %v, want nil", *decoded.Refresh)
				}
				return
			}
			if decoded.Refresh == nil || *decoded.Refresh != tc.want {
				t.Fatalf("Refresh = %v, want %v", decoded.Refresh, tc.want)
			}
		})
	}
}
