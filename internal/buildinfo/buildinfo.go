// Package buildinfo holds the release version stamped in at build time.
package buildinfo

// Version is set with
//
//	-ldflags "-X github.com/jeffwnelson/sodatracker/internal/buildinfo.Version=v1.2.3"
//
// by the Dockerfile and Makefile, from the cocogitto release tag (or
// `git describe` between releases). Unstamped builds report "dev".
var Version = "dev"
