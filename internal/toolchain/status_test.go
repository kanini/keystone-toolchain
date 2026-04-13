package toolchain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kanini/keystone-toolchain/internal/runtime"
)

func TestBuildStatusReportWithoutStateMarksUnknown(t *testing.T) {
	home := t.TempDir()
	t.Setenv("PATH", "")
	ctx := &runtime.Context{
		HomeDir: home,
		Config: runtime.Config{
			ManagedBinDir: filepath.Join(home, ".keystone", "toolchain", "active", "bin"),
			StateDir:      filepath.Join(home, ".keystone", "toolchain", "state"),
		},
	}
	manifest := Manifest{
		Schema:        "kstoolchain.adapter/v1alpha1",
		ManagedBinDir: ctx.Config.ManagedBinDir,
		Repos: []RepoAdapter{
			{
				RepoID:          "keystone-memory",
				RepoPath:        "/tmp/keystone-memory",
				ExpectedOutputs: []string{"ksmem"},
				DirtyPolicy:     "fail_closed",
				ReleaseUnit:     "repo",
				Status:          "candidate",
			},
		},
	}

	report := BuildStatusReport(ctx, manifest, PersistedState{}, filepath.Join(ctx.Config.StateDir, "current.json"), false)
	if report.Summary.Overall != StateUnknown {
		t.Fatalf("unexpected overall state: %s", report.Summary.Overall)
	}
	if got := report.Repos[0].State; got != StateUnknown {
		t.Fatalf("unexpected repo state: %s", got)
	}
	if report.Repos[0].Reason == "" {
		t.Fatal("expected reason")
	}
}

func TestBuildStatusReportMarksShadowedPath(t *testing.T) {
	home := t.TempDir()
	pathDir := filepath.Join(home, "path-bin")
	if err := os.MkdirAll(pathDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	shadowed := filepath.Join(pathDir, "ksmem")
	if err := os.WriteFile(shadowed, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write shadowed binary: %v", err)
	}
	t.Setenv("PATH", pathDir)

	ctx := &runtime.Context{
		HomeDir: home,
		Config: runtime.Config{
			ManagedBinDir: filepath.Join(home, ".keystone", "toolchain", "active", "bin"),
			StateDir:      filepath.Join(home, ".keystone", "toolchain", "state"),
		},
	}
	manifest := Manifest{
		Schema:        "kstoolchain.adapter/v1alpha1",
		ManagedBinDir: ctx.Config.ManagedBinDir,
		Repos: []RepoAdapter{
			{
				RepoID:          "keystone-memory",
				RepoPath:        "/tmp/keystone-memory",
				ExpectedOutputs: []string{"ksmem"},
				DirtyPolicy:     "fail_closed",
				ReleaseUnit:     "repo",
				Status:          "candidate",
			},
		},
	}
	persisted := PersistedState{
		Repos: []PersistedRepoState{
			{RepoID: "keystone-memory", State: StateCurrent},
		},
	}

	report := BuildStatusReport(ctx, manifest, persisted, filepath.Join(ctx.Config.StateDir, "current.json"), true)
	if report.Summary.Overall != StateShadowed {
		t.Fatalf("unexpected overall state: %s", report.Summary.Overall)
	}
	output := report.Repos[0].Outputs[0]
	if output.State != StateShadowed {
		t.Fatalf("unexpected output state: %s", output.State)
	}
	if !strings.Contains(output.Reason, "PATH resolves") {
		t.Fatalf("unexpected output reason: %q", output.Reason)
	}
}

func TestRenderStatusTextIncludesSummary(t *testing.T) {
	report := StatusReport{
		Summary: StatusSummary{
			Overall:          StateUnknown,
			RepoCount:        1,
			OutputCount:      1,
			BlockedRepoCount: 0,
		},
		ManagedBinDir: "/tmp/bin",
		StateFile:     "/tmp/current.json",
		Repos: []RepoStatus{
			{
				RepoID:        "keystone-memory",
				AdapterStatus: "candidate",
				State:         StateUnknown,
				RepoPath:      "/tmp/repo",
				Outputs: []OutputStatus{
					{Name: "ksmem", State: StateUnknown, ExpectedPath: "/tmp/bin/ksmem"},
				},
			},
		},
	}

	lines := RenderStatusText(report)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "Suite: UNKNOWN") {
		t.Fatalf("unexpected text: %s", joined)
	}
	if !strings.Contains(joined, "keystone-memory  UNKNOWN") {
		t.Fatalf("unexpected text: %s", joined)
	}
}
