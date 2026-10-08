// Package version holds the release version. tagpr updates it.
package version

// Version is the current release.
const Version = "1.7.2"

// Source is "release" in the binaries the release workflow builds (its
// -ldflags set it), and empty in any other build: make install, go install.
var Source string
