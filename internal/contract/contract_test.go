package contract

import (
	"strings"
	"testing"
)

func TestVersionStringIncludesToolName(t *testing.T) {
	oldVersion := KSToolchainVersion
	oldCommit := BuildCommit
	oldDate := BuildDate
	oldSource := BuildSource
	t.Cleanup(func() {
		KSToolchainVersion = oldVersion
		BuildCommit = oldCommit
		BuildDate = oldDate
		BuildSource = oldSource
	})

	KSToolchainVersion = "1.2.3"
	BuildCommit = "abcdef1"
	BuildDate = "2026-04-12T00:00:00Z"
	BuildSource = "test"

	got := VersionString()
	if !strings.Contains(got, "kstoolchain version 1.2.3") {
		t.Fatalf("expected version line, got %q", got)
	}
	if !strings.Contains(got, "commit=abcdef1") {
		t.Fatalf("expected commit in version line, got %q", got)
	}
}

func TestCompareCommitsWarnsOnMismatch(t *testing.T) {
	warning := compareCommits("kstoolchain", "/tmp/repo", "abc1234", "def5678")
	if warning == nil {
		t.Fatal("expected warning")
	}
	if warning.Code != WarningStaleBinary {
		t.Fatalf("unexpected code: %s", warning.Code)
	}
	if !strings.Contains(warning.Message, "built from abc1234 but repo is at def5678") {
		t.Fatalf("unexpected message: %q", warning.Message)
	}
}

func TestCompareCommitsAllowsMatchingPrefixes(t *testing.T) {
	warning := compareCommits("kstoolchain", "/tmp/repo", "abc1234", "abc123456789")
	if warning != nil {
		t.Fatalf("expected no warning, got %#v", warning)
	}
}
