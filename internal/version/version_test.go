package version

import (
	"strings"
	"testing"
)

func TestInfo_Defaults(t *testing.T) {
	info := Info()
	if info.Version != "dev" {
		t.Errorf("version = %q, want dev", info.Version)
	}
	if info.Commit != "unknown" {
		t.Errorf("commit = %q, want unknown", info.Commit)
	}
	if info.Date != "unknown" {
		t.Errorf("date = %q, want unknown", info.Date)
	}
}

func TestBuildInfo_String(t *testing.T) {
	info := BuildInfo{Version: "v1.0.0", Commit: "abc123", Date: "2026-01-01"}
	s := info.String()
	if !strings.Contains(s, "v1.0.0") {
		t.Errorf("string should contain version: %s", s)
	}
	if !strings.Contains(s, "abc123") {
		t.Errorf("string should contain commit: %s", s)
	}
	if !strings.Contains(s, "2026-01-01") {
		t.Errorf("string should contain date: %s", s)
	}
}
