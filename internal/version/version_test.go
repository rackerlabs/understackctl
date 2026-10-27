package version

import (
	"runtime/debug"
	"testing"
)

func TestNormalize(t *testing.T) {
	tests := map[string]string{
		"0.1.0":                      "v0.1.0",
		"v0.1.0":                     "v0.1.0",
		"0.1.0-rc.1":                 "v0.1.0-rc.1",
		"0.0.1-snapshot-630c1e9":     "v0.0.1-snapshot-630c1e9",
		"dev":                        "dev",
		"":                           "",
		"v0.0.0-20260906032901-abcd": "v0.0.0-20260906032901-abcd",
		// `git describe --always` in a repo with no tags yields a bare SHA,
		// which must not be dressed up as a version.
		"630c1e9":       "630c1e9",
		"630c1e9-dirty": "630c1e9-dirty",
		"1234567":       "1234567",
	}

	for in, want := range tests {
		if got := normalize(in); got != want {
			t.Errorf("normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGetFallsBackToDefaults(t *testing.T) {
	// The test binary has no linker-stamped values, so Get must still return
	// something usable for every field.
	info := Get()

	if info.Version == "" {
		t.Error("Version is empty")
	}
	if info.Commit == "" {
		t.Error("Commit is empty")
	}
	if info.Date == "" {
		t.Error("Date is empty")
	}
	if info.GoVersion == "" {
		t.Error("GoVersion is empty")
	}
	if info.Platform == "" {
		t.Error("Platform is empty")
	}
}

func TestFillFromBuildInfoUsesModuleVersion(t *testing.T) {
	bi := &debug.BuildInfo{}
	bi.Main.Version = "v0.1.0"

	info := Info{}
	info.fillFromBuildInfo(bi)

	if info.Version != "v0.1.0" {
		t.Errorf("Version = %q, want v0.1.0", info.Version)
	}
}

func TestFillFromBuildInfoIgnoresDevelPlaceholder(t *testing.T) {
	bi := &debug.BuildInfo{}
	bi.Main.Version = "(devel)"

	info := Info{}
	info.fillFromBuildInfo(bi)

	if info.Version != "" {
		t.Errorf("Version = %q, want empty so the caller falls back to dev", info.Version)
	}
}

func TestFillFromBuildInfoVCSSettings(t *testing.T) {
	bi := &debug.BuildInfo{
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "630c1e90354d1234567890"},
			{Key: "vcs.time", Value: "2026-09-06T03:29:01Z"},
			{Key: "vcs.modified", Value: "true"},
		},
	}

	info := Info{}
	info.fillFromBuildInfo(bi)

	if info.Commit != "630c1e9-dirty" {
		t.Errorf("Commit = %q, want 630c1e9-dirty", info.Commit)
	}
	if info.Date != "2026-09-06T03:29:01Z" {
		t.Errorf("Date = %q, want 2026-09-06T03:29:01Z", info.Date)
	}
}

func TestFillFromBuildInfoDoesNotOverrideLinkerValues(t *testing.T) {
	bi := &debug.BuildInfo{
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "aaaaaaaaaaaa"},
			{Key: "vcs.time", Value: "2020-01-01T00:00:00Z"},
		},
	}
	bi.Main.Version = "v9.9.9"

	// Simulates a release build where the linker set everything.
	info := Info{Version: "0.1.0", Commit: "630c1e9", Date: "2026-09-06T03:29:01Z"}
	info.fillFromBuildInfo(bi)

	if info.Version != "0.1.0" {
		t.Errorf("Version = %q, want the linker value 0.1.0", info.Version)
	}
	if info.Commit != "630c1e9" {
		t.Errorf("Commit = %q, want the linker value 630c1e9", info.Commit)
	}
	if info.Date != "2026-09-06T03:29:01Z" {
		t.Errorf("Date = %q, want the linker value 2026-09-06T03:29:01Z", info.Date)
	}
}
