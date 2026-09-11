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
