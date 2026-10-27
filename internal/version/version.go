// Package version reports the build version of understackctl.
//
// Release builds have these values stamped in by the linker (see
// .goreleaser.yaml). Builds produced any other way -- `go install
// github.com/rackerlabs/understackctl@v0.1.0`, or `go build` inside a
// checkout -- fall back to the module and VCS metadata that the Go toolchain
// records in the binary, so that `understackctl version` is useful in a bug
// report no matter how the binary was obtained.
package version

import (
	"fmt"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
)

// Set via -ldflags "-X github.com/rackerlabs/understackctl/internal/version.<name>=...".
var (
	version string
	commit  string
	date    string
)

const unknown = "unknown"

// Info describes the running binary.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Date      string `json:"date"`
	GoVersion string `json:"goVersion"`
	Platform  string `json:"platform"`
}

// Get resolves build information, preferring linker-stamped values and
// falling back to the toolchain's embedded build info.
func Get() Info {
	info := Info{
		Version:   version,
		Commit:    commit,
		Date:      date,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}

	if bi, ok := debug.ReadBuildInfo(); ok {
		info.fillFromBuildInfo(bi)
	}

	if info.Version == "" {
		info.Version = "dev"
	}
	if info.Commit == "" {
		info.Commit = unknown
	}
	if info.Date == "" {
		info.Date = unknown
	}
	info.Version = normalize(info.Version)

	return info
}

// semverish matches a leading MAJOR.MINOR, which is enough to tell a version
// from the bare commit SHA that `git describe --always` produces when the
// repository has no tags.
var semverish = regexp.MustCompile(`^[0-9]+\.[0-9]+`)

// normalize adds the "v" prefix that GoReleaser's .Version template strips, so
// that a release build and `go install ...@v0.1.0` report the same string.
// Anything that is not recognisably a version -- "dev", a commit SHA -- is
// left alone.
func normalize(v string) string {
	if semverish.MatchString(v) {
		return "v" + v
	}
	return v
}

// fillFromBuildInfo populates any field that the linker did not set.
func (i *Info) fillFromBuildInfo(bi *debug.BuildInfo) {
	// bi.Main.Version is the module version for `go install pkg@version`, and
	// "(devel)" for a build from a local checkout. Only the former is useful.
	if i.Version == "" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		i.Version = bi.Main.Version
	}

	var revision, modified string
	for _, setting := range bi.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.time":
			if i.Date == "" {
				i.Date = setting.Value
			}
		case "vcs.modified":
			modified = setting.Value
		}
	}

	if i.Commit == "" && revision != "" {
		i.Commit = shortCommit(revision)
		if modified == "true" {
			i.Commit += "-dirty"
		}
	}
}

func shortCommit(revision string) string {
	if len(revision) > 7 {
		return revision[:7]
	}
	return revision
}

// String renders the one-line form used by `understackctl --version`.
func (i Info) String() string {
	return fmt.Sprintf("%s (commit %s, built %s, %s %s)",
		i.Version, i.Commit, i.Date, i.GoVersion, i.Platform)
}

// Multiline renders the block form used by `understackctl version`.
func (i Info) Multiline() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Version:    %s\n", i.Version)
	fmt.Fprintf(&sb, "Commit:     %s\n", i.Commit)
	fmt.Fprintf(&sb, "Built:      %s\n", i.Date)
	fmt.Fprintf(&sb, "Go version: %s\n", i.GoVersion)
	fmt.Fprintf(&sb, "Platform:   %s\n", i.Platform)
	return sb.String()
}
