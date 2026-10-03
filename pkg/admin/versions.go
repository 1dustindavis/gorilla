package admin

import (
	"fmt"
	"math/big"
	"strings"
)

func parseNumericVersion(version string) ([]*big.Int, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return nil, fmt.Errorf("version is required")
	}

	parts := strings.Split(version, ".")
	segments := make([]*big.Int, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			return nil, fmt.Errorf("unsupported version %q", version)
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return nil, fmt.Errorf("unsupported version %q", version)
			}
		}
		value := new(big.Int)
		if _, ok := value.SetString(part, 10); !ok {
			return nil, fmt.Errorf("unsupported version %q", version)
		}
		segments = append(segments, value)
	}
	return segments, nil
}

// compareVersions compares dotted numeric versions. Missing trailing segments
// are treated as zero, so 1, 1.0, and 1.0.0 compare equivalently.
func compareVersions(a, b string) (int, error) {
	aSegments, err := parseNumericVersion(a)
	if err != nil {
		return 0, err
	}
	bSegments, err := parseNumericVersion(b)
	if err != nil {
		return 0, err
	}

	length := len(aSegments)
	if len(bSegments) > length {
		length = len(bSegments)
	}
	zero := big.NewInt(0)
	for i := 0; i < length; i++ {
		aValue := zero
		bValue := zero
		if i < len(aSegments) {
			aValue = aSegments[i]
		}
		if i < len(bSegments) {
			bValue = bSegments[i]
		}
		if cmp := aValue.Cmp(bValue); cmp != 0 {
			return cmp, nil
		}
	}
	return 0, nil
}
