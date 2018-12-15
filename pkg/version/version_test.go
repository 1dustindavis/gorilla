/*
MIT License
Copyright (c) 2017 Kolide
Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:
The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.
THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package version

import (
	"testing"
	"time"
)

// TestVersion verifies that the corect strings are returned
func TestVersion(t *testing.T) {
	now := time.Now().String()
	version = "5.5.1"
	branch = "testing_feature_branch"
	revision = "0b4ecc2143f34e5e38f2ea9d2e936a7cfa71bb18"
	goVersion = "go2.43"
	buildDate = now

	info := Version()

	if have, want := info.Version, version; have != want {
		t.Errorf("have %s, want %s", have, want)
	}

	if have, want := info.Branch, branch; have != want {
		t.Errorf("have %s, want %s", have, want)
	}

	if have, want := info.Revision, revision; have != want {
		t.Errorf("have %s, want %s", have, want)
	}

	if have, want := info.GoVersion, goVersion; have != want {
		t.Errorf("have %s, want %s", have, want)
	}

	if have, want := info.BuildDate, buildDate; have != want {
		t.Errorf("have %s, want %s", have, want)
	}
}

// ExamplePrint tests the output of Print()
func ExamplePrint() {
	// Set up what we expect
	appName = "CoolAppplication"
	version = "v9.4.2.7-beta324"

	// Run the function
	Print()
	// Output:
	// CoolAppplication v9.4.2.7-beta324
}

// ExamplePrintFull tests the output of PrintFull()
func ExamplePrintFull() {
	// Set up what we expect
	appName = "OkayAppplication"
	version = "ver12.0"
	branch = "branch_branch"
	revision = "0a7aca6523e34c5a38f2da9d2a975a7cfa21ba42"
	buildDate = "2018-09-07 07:30:50.390193 -0700 PDT m=+0.003381227"
	goVersion = "go0.2"

	// Run the function
	PrintFull()
	// Output:
	// OkayAppplication ver12.0
	//   branch: 	branch_branch
	//   revision: 	0a7aca6523e34c5a38f2da9d2a975a7cfa21ba42
	//   build date: 	2018-09-07 07:30:50.390193 -0700 PDT m=+0.003381227
	//   go version: 	go0.2
}
