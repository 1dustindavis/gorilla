package download

import "testing"

// TestVerify tests hash comparison
func TestVerify(t *testing.T) {
	// Setup test values
	testFile := "testdata/hashtest.txt"
	validHash := "dca48f4e34541c52d12351479454b3af6d87d8dc23ec48f68962f062d8703de3"
	invalidHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	// Run the code
	validTest := Verify(testFile, validHash)
	invalidTest := Verify(testFile, invalidHash)

	// Compare output
	if have, want := validTest, true; have != want {
		t.Errorf("have %v, want %v", have, want)
	}

	// Compare output
	if have, want := invalidTest, false; have != want {
		t.Errorf("have %v, want %v", have, want)
	}
}
