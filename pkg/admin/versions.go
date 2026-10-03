package admin

import (
	"fmt"
	"math/big"
	"strings"
)

type versionToken struct {
	text      string
	numeric   bool
	number    *big.Int
	preRank   int
	isPrerelease bool
}

var prereleaseRanks = map[string]int{
	"dev":     1,
	"alpha":   2,
	"a":       2,
	"beta":    3,
	"b":       3,
	"preview": 4,
	"pre":     4,
	"rc":      5,
}

func parseVersion(version string) ([]versionToken, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return nil, fmt.Errorf("version is required")
	}
	if len(version) > 1 && (version[0] == 'v' || version[0] == 'V') && version[1] >= '0' && version[1] <= '9' {
		version = version[1:]
	}

	var tokens []versionToken
	hasNumeric := false
	for i := 0; i < len(version); {
		c := version[i]
		if c == '.' || c == '-' || c == '_' || c == '+' {
			i++
			continue
		}
		if c >= '0' && c <= '9' {
			start := i
			for i < len(version) && version[i] >= '0' && version[i] <= '9' {
				i++
			}
			text := version[start:i]
			number := new(big.Int)
			if _, ok := number.SetString(text, 10); !ok {
				return nil, fmt.Errorf("unsupported version %q", version)
			}
			tokens = append(tokens, versionToken{text: text, numeric: true, number: number})
			hasNumeric = true
			continue
		}
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			start := i
			for i < len(version) {
				c = version[i]
				if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
					break
				}
				i++
			}
			text := strings.ToLower(version[start:i])
			rank, prerelease := prereleaseRanks[text]
			tokens = append(tokens, versionToken{text: text, preRank: rank, isPrerelease: prerelease})
			continue
		}
		return nil, fmt.Errorf("unsupported version %q", version)
	}
	if !hasNumeric {
		return nil, fmt.Errorf("unsupported version %q", version)
	}
	return tokens, nil
}

// compareVersions compares concrete versions by alternating numeric and text
// runs. Numeric runs compare numerically; text runs compare case-insensitively.
// Common prerelease markers sort below a corresponding final release. Missing
// trailing numeric zero runs are ignored, so 1, 1.0, and 1.0.0 compare equally.
func compareVersions(a, b string) (int, error) {
	aTokens, err := parseVersion(a)
	if err != nil {
		return 0, err
	}
	bTokens, err := parseVersion(b)
	if err != nil {
		return 0, err
	}

	limit := len(aTokens)
	if len(bTokens) < limit {
		limit = len(bTokens)
	}
	for i := 0; i < limit; i++ {
		cmp := compareVersionToken(aTokens[i], bTokens[i])
		if cmp != 0 {
			return cmp, nil
		}
	}
	if len(aTokens) == len(bTokens) {
		return 0, nil
	}
	if len(aTokens) > limit {
		return compareVersionRemainder(aTokens[limit:]), nil
	}
	return -compareVersionRemainder(bTokens[limit:]), nil
}

func compareVersionToken(a, b versionToken) int {
	if a.numeric && b.numeric {
		return a.number.Cmp(b.number)
	}
	if a.numeric != b.numeric {
		if a.numeric {
			return 1
		}
		return -1
	}
	if a.isPrerelease && b.isPrerelease && a.preRank != b.preRank {
		if a.preRank < b.preRank {
			return -1
		}
		return 1
	}
	return strings.Compare(a.text, b.text)
}

func compareVersionRemainder(tokens []versionToken) int {
	for _, token := range tokens {
		if token.numeric {
			if token.number.Sign() != 0 {
				return 1
			}
			continue
		}
		if token.isPrerelease {
			return -1
		}
		return 1
	}
	return 0
}
