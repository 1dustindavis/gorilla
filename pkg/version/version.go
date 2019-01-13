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

/*
Package version provides utilities for displaying version information about a Go application.

To use this package, a program would set the package variables at build time, using the
-ldflags go build flag.

Example:
	go build -ldflags "-X github.com/kolide/kit/version.version=1.0.0"

Available values and defaults to use with ldflags:

	version   = "unknown"
	branch    = "unknown"
	revision  = "unknown"
	goVersion = "unknown"
	buildDate = "unknown"
	appName   = "unknown"
*/
package version

import (
	"fmt"
)

// These values are private which ensures they can only be set with the build flags.
var (
	version   = "unknown"
	branch    = "unknown"
	revision  = "unknown"
	goVersion = "unknown"
	buildDate = "unknown"
	appName   = "unknown"
)

// Info is a structure with version build information about the current application.
type Info struct {
	Version   string `json:"version"`
	Branch    string `json:"branch"`
	Revision  string `json:"revision"`
	GoVersion string `json:"go_version"`
	BuildDate string `json:"build_date"`
}

// Version returns a structure with the current version information.
func Version() Info {
	return Info{
		Version:   version,
		Branch:    branch,
		Revision:  revision,
		GoVersion: goVersion,
		BuildDate: buildDate,
	}
}

// Print outputs the application name and version string.
func Print() {
	v := Version()
	fmt.Printf("%s %s\n", appName, v.Version)
}

// PrintFull prints the application name and detailed version information.
func PrintFull() {
	v := Version()
	fmt.Printf("%s %s\n", appName, v.Version)
	fmt.Printf("  branch: \t%s\n", v.Branch)
	fmt.Printf("  revision: \t%s\n", v.Revision)
	fmt.Printf("  build date: \t%s\n", v.BuildDate)
	fmt.Printf("  go version: \t%s\n", v.GoVersion)
}
