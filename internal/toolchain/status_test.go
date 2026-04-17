package toolchain

import (
	"os"
	"os/exec"
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

	report := BuildStatusReport(ctx, manifest, PersistedState{}, filepath.Join(ctx.Config.StateDir, "current.json"), false, false)
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
	repoPath := testGitRepo(t, home)
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
				RepoPath:        repoPath,
				ExpectedOutputs: []string{"ksmem"},
				DirtyPolicy:     "fail_closed",
				ReleaseUnit:     "repo",
				Status:          AdapterStatusReady,
			},
		},
	}
	persisted := PersistedState{
		Repos: []PersistedRepoState{
			{RepoID: "keystone-memory", State: StateCurrent},
		},
	}

	report := BuildStatusReport(ctx, manifest, persisted, filepath.Join(ctx.Config.StateDir, "current.json"), true, false)
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

func TestBuildStatusReportIgnoresCandidateRepoInOverall(t *testing.T) {
	home := t.TempDir()
	hubRepo := testGitRepo(t, home)
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
				RepoID:          "keystone-hub",
				RepoPath:        hubRepo,
				ExpectedOutputs: []string{"kshub"},
				DirtyPolicy:     DirtyPolicyFailClosed,
				ReleaseUnit:     ReleaseUnitRepo,
				Status:          AdapterStatusReady,
			},
			{
				RepoID:          "keystone-memory",
				RepoPath:        filepath.Join(home, "memory"),
				ExpectedOutputs: []string{"ksmem"},
				DirtyPolicy:     DirtyPolicyFailClosed,
				ReleaseUnit:     ReleaseUnitRepo,
				Status:          AdapterStatusCandidate,
			},
		},
	}
	persisted := PersistedState{
		Schema:        PersistedStateSchema,
		ManagedBinDir: ctx.Config.ManagedBinDir,
		Repos: []PersistedRepoState{
			{RepoID: "keystone-hub", State: StateCurrent},
		},
	}

	t.Setenv("PATH", ctx.Config.ManagedBinDir)
	if err := os.MkdirAll(ctx.Config.ManagedBinDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ctx.Config.ManagedBinDir, "kshub"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write kshub: %v", err)
	}

	report := BuildStatusReport(ctx, manifest, persisted, filepath.Join(ctx.Config.StateDir, "current.json"), true, false)
	if got := report.Summary.Overall; got != StateCurrent {
		t.Fatalf("unexpected overall state: %s", got)
	}
}

func TestBuildStatusReportKeepsDirtySkippedWhenHeadMoves(t *testing.T) {
	home := t.TempDir()

	hubRepo := filepath.Join(home, "hub")
	if err := os.MkdirAll(hubRepo, 0o755); err != nil {
		t.Fatalf("mkdir hub repo: %v", err)
	}
	cmd := exec.Command("git", "init")
	cmd.Dir = hubRepo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, string(out))
	}
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = hubRepo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git config email: %v\n%s", err, string(out))
	}
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = hubRepo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git config name: %v\n%s", err, string(out))
	}
	if err := os.WriteFile(filepath.Join(hubRepo, "README.md"), []byte("one\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	cmd = exec.Command("git", "add", "README.md")
	cmd.Dir = hubRepo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, string(out))
	}
	cmd = exec.Command("git", "commit", "-m", "init")
	cmd.Dir = hubRepo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, string(out))
	}
	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = hubRepo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git rev-parse: %v\n%s", err, string(out))
	}
	activeBuild := strings.TrimSpace(string(out))

	if err := os.WriteFile(filepath.Join(hubRepo, "README.md"), []byte("two\n"), 0o644); err != nil {
		t.Fatalf("write README second: %v", err)
	}
	cmd = exec.Command("git", "add", "README.md")
	cmd.Dir = hubRepo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add second: %v\n%s", err, string(out))
	}
	cmd = exec.Command("git", "commit", "-m", "second")
	cmd.Dir = hubRepo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit second: %v\n%s", err, string(out))
	}
	if err := os.WriteFile(filepath.Join(hubRepo, "README.md"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatalf("write dirty README: %v", err)
	}

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
				RepoID:          "keystone-hub",
				RepoPath:        hubRepo,
				ExpectedOutputs: []string{"kshub"},
				DirtyPolicy:     DirtyPolicyFailClosed,
				ReleaseUnit:     ReleaseUnitRepo,
				Status:          AdapterStatusReady,
			},
		},
	}
	persisted := PersistedState{
		Schema:        PersistedStateSchema,
		ManagedBinDir: ctx.Config.ManagedBinDir,
		Repos: []PersistedRepoState{
			{RepoID: "keystone-hub", State: StateDirtySkipped, ActiveBuild: activeBuild, Reason: "repo has uncommitted changes; sync is fail_closed in v1"},
		},
	}

	t.Setenv("PATH", "")
	report := BuildStatusReport(ctx, manifest, persisted, filepath.Join(ctx.Config.StateDir, "current.json"), true, false)
	if got := report.Repos[0].State; got != StateDirtySkipped {
		t.Fatalf("unexpected repo state: %s", got)
	}
}

func TestBuildStatusReportProjectsUnresolvedPrePromotionAttemptWithoutRewritingRepoRows(t *testing.T) {
	home := t.TempDir()
	repoPath := testGitRepo(t, home)
	head, ok := lookupRepoHead(repoPath)
	if !ok {
		t.Fatal("expected repo HEAD")
	}
	t.Setenv("PATH", filepath.Join(home, ".keystone", "toolchain", "active", "bin"))

	ctx := &runtime.Context{
		HomeDir: home,
		Config: runtime.Config{
			ManagedBinDir: filepath.Join(home, ".keystone", "toolchain", "active", "bin"),
			StateDir:      filepath.Join(home, ".keystone", "toolchain", "state"),
		},
	}
	if err := os.MkdirAll(ctx.Config.ManagedBinDir, 0o755); err != nil {
		t.Fatalf("mkdir managed bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ctx.Config.ManagedBinDir, "kshub"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write managed binary: %v", err)
	}
	manifest := Manifest{
		Schema:        "kstoolchain.adapter/v1alpha1",
		ManagedBinDir: ctx.Config.ManagedBinDir,
		Repos: []RepoAdapter{
			{
				RepoID:          "keystone-hub",
				RepoPath:        repoPath,
				ExpectedOutputs: []string{"kshub"},
				DirtyPolicy:     DirtyPolicyFailClosed,
				ReleaseUnit:     ReleaseUnitRepo,
				Status:          AdapterStatusReady,
			},
		},
	}
	persisted := PersistedState{
		Schema:             PersistedStateSchema,
		ManagedBinDir:      ctx.Config.ManagedBinDir,
		CommittedAttemptID: "commit-1",
		Repos: []PersistedRepoState{
			{
				RepoID:                "keystone-hub",
				State:                 StateCurrent,
				RepoHead:              head,
				LastAttemptSourceKind: SourceKindCleanHead,
				ActiveBuild:           head,
				ActiveSourceKind:      SourceKindCleanHead,
			},
		},
	}
	if appErr := saveAttemptArtifact(ctx, SyncAttemptArtifact{
		Schema:       SyncAttemptSchema,
		AttemptID:    "attempt-2",
		StartedAt:    "2026-04-17T21:00:00Z",
		ReadyRepoIDs: []string{"keystone-hub"},
		Phase:        AttemptPhasePrePromotion,
	}); appErr != nil {
		t.Fatalf("save attempt artifact: %v", appErr)
	}

	report := BuildStatusReport(ctx, manifest, persisted, filepath.Join(ctx.Config.StateDir, "current.json"), true, false)
	if got := report.Repos[0].State; got != StateCurrent {
		t.Fatalf("repo row must stay anchored in current.json, got %s", got)
	}
	if report.AttemptIntegrity == nil {
		t.Fatal("expected attempt integrity overlay")
	}
	if got := report.AttemptIntegrity.State; got != AttemptIntegrityPrePromotion {
		t.Fatalf("unexpected attempt integrity state: %s", got)
	}
	if got := report.Summary.Overall; got != StateUnknown {
		t.Fatalf("expected suite overall to lose trusted CURRENT, got %s", got)
	}
	if got := StatusExitCode(report); got != 1 {
		t.Fatalf("expected non-zero status exit when attempt integrity is unresolved, got %d", got)
	}
}

func TestBuildStatusReportProjectsUnresolvedPromotionAttemptAsSuiteLevelBlocker(t *testing.T) {
	home := t.TempDir()
	repoPath := testGitRepo(t, home)
	head, ok := lookupRepoHead(repoPath)
	if !ok {
		t.Fatal("expected repo HEAD")
	}
	t.Setenv("PATH", filepath.Join(home, ".keystone", "toolchain", "active", "bin"))

	ctx := &runtime.Context{
		HomeDir: home,
		Config: runtime.Config{
			ManagedBinDir: filepath.Join(home, ".keystone", "toolchain", "active", "bin"),
			StateDir:      filepath.Join(home, ".keystone", "toolchain", "state"),
		},
	}
	if err := os.MkdirAll(ctx.Config.ManagedBinDir, 0o755); err != nil {
		t.Fatalf("mkdir managed bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ctx.Config.ManagedBinDir, "kshub"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write managed binary: %v", err)
	}
	manifest := Manifest{
		Schema:        "kstoolchain.adapter/v1alpha1",
		ManagedBinDir: ctx.Config.ManagedBinDir,
		Repos: []RepoAdapter{
			{
				RepoID:          "keystone-hub",
				RepoPath:        repoPath,
				ExpectedOutputs: []string{"kshub"},
				DirtyPolicy:     DirtyPolicyFailClosed,
				ReleaseUnit:     ReleaseUnitRepo,
				Status:          AdapterStatusReady,
			},
		},
	}
	persisted := PersistedState{
		Schema:             PersistedStateSchema,
		ManagedBinDir:      ctx.Config.ManagedBinDir,
		CommittedAttemptID: "commit-1",
		Repos: []PersistedRepoState{
			{
				RepoID:                "keystone-hub",
				State:                 StateCurrent,
				RepoHead:              head,
				LastAttemptSourceKind: SourceKindCleanHead,
				ActiveBuild:           head,
				ActiveSourceKind:      SourceKindCleanHead,
			},
		},
	}
	if appErr := saveAttemptArtifact(ctx, SyncAttemptArtifact{
		Schema:                 SyncAttemptSchema,
		AttemptID:              "attempt-2",
		StartedAt:              "2026-04-17T21:00:00Z",
		ReadyRepoIDs:           []string{"keystone-hub"},
		Phase:                  AttemptPhasePrePromotion,
		CarriedUnresolvedPhase: AttemptPhasePromotionOrLater,
	}); appErr != nil {
		t.Fatalf("save attempt artifact: %v", appErr)
	}

	report := BuildStatusReport(ctx, manifest, persisted, filepath.Join(ctx.Config.StateDir, "current.json"), true, false)
	if report.AttemptIntegrity == nil {
		t.Fatal("expected attempt integrity overlay")
	}
	if got := report.AttemptIntegrity.State; got != AttemptIntegrityPromotionLate {
		t.Fatalf("unexpected attempt integrity state: %s", got)
	}
	if got := report.Repos[0].State; got != StateCurrent {
		t.Fatalf("repo row must remain CURRENT, got %s", got)
	}
	lines := strings.Join(RenderStatusText(report), "\n")
	if !strings.Contains(lines, "Attempt integrity: "+AttemptIntegrityPromotionLate) {
		t.Fatalf("expected attempt integrity in text render: %s", lines)
	}
	if !strings.Contains(lines, "run `kstoolchain sync` again") {
		t.Fatalf("expected rerun guidance in text render: %s", lines)
	}
}

func TestBuildStatusReportInvalidAttemptArtifactFailsClosed(t *testing.T) {
	home := t.TempDir()
	repoPath := testGitRepo(t, home)
	head, ok := lookupRepoHead(repoPath)
	if !ok {
		t.Fatal("expected repo HEAD")
	}
	t.Setenv("PATH", filepath.Join(home, ".keystone", "toolchain", "active", "bin"))

	ctx := &runtime.Context{
		HomeDir: home,
		Config: runtime.Config{
			ManagedBinDir: filepath.Join(home, ".keystone", "toolchain", "active", "bin"),
			StateDir:      filepath.Join(home, ".keystone", "toolchain", "state"),
		},
	}
	if err := os.MkdirAll(ctx.Config.ManagedBinDir, 0o755); err != nil {
		t.Fatalf("mkdir managed bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ctx.Config.ManagedBinDir, "kshub"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write managed binary: %v", err)
	}
	if err := os.MkdirAll(ctx.Config.StateDir, 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ctx.Config.StateDir, syncAttemptFileName), []byte("{invalid\n"), 0o644); err != nil {
		t.Fatalf("write invalid attempt artifact: %v", err)
	}
	manifest := Manifest{
		Schema:        "kstoolchain.adapter/v1alpha1",
		ManagedBinDir: ctx.Config.ManagedBinDir,
		Repos: []RepoAdapter{
			{
				RepoID:          "keystone-hub",
				RepoPath:        repoPath,
				ExpectedOutputs: []string{"kshub"},
				DirtyPolicy:     DirtyPolicyFailClosed,
				ReleaseUnit:     ReleaseUnitRepo,
				Status:          AdapterStatusReady,
			},
		},
	}
	persisted := PersistedState{
		Schema:        PersistedStateSchema,
		ManagedBinDir: ctx.Config.ManagedBinDir,
		Repos: []PersistedRepoState{
			{
				RepoID:                "keystone-hub",
				State:                 StateCurrent,
				RepoHead:              head,
				LastAttemptSourceKind: SourceKindCleanHead,
				ActiveBuild:           head,
				ActiveSourceKind:      SourceKindCleanHead,
			},
		},
	}

	report := BuildStatusReport(ctx, manifest, persisted, filepath.Join(ctx.Config.StateDir, "current.json"), true, false)
	if report.AttemptIntegrity == nil {
		t.Fatal("expected invalid attempt artifact to fail closed")
	}
	if got := report.AttemptIntegrity.State; got != AttemptIntegrityArtifactBad {
		t.Fatalf("unexpected attempt integrity state: %s", got)
	}
	if got := report.Summary.Overall; got != StateUnknown {
		t.Fatalf("expected invalid attempt artifact to suppress trusted CURRENT, got %s", got)
	}
	if got := report.Repos[0].State; got != StateCurrent {
		t.Fatalf("repo row must remain current.json truth, got %s", got)
	}
}

func TestBuildStatusReportUnknownAttemptSchemaFailsClosed(t *testing.T) {
	home := t.TempDir()
	repoPath := testGitRepo(t, home)
	head, ok := lookupRepoHead(repoPath)
	if !ok {
		t.Fatal("expected repo HEAD")
	}
	t.Setenv("PATH", filepath.Join(home, ".keystone", "toolchain", "active", "bin"))

	ctx := &runtime.Context{
		HomeDir: home,
		Config: runtime.Config{
			ManagedBinDir: filepath.Join(home, ".keystone", "toolchain", "active", "bin"),
			StateDir:      filepath.Join(home, ".keystone", "toolchain", "state"),
		},
	}
	if err := os.MkdirAll(ctx.Config.ManagedBinDir, 0o755); err != nil {
		t.Fatalf("mkdir managed bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ctx.Config.ManagedBinDir, "kshub"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write managed binary: %v", err)
	}
	if err := os.MkdirAll(ctx.Config.StateDir, 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	raw := `{
  "schema": "kstoolchain.sync-attempt/v9alpha9",
  "attempt_id": "attempt-2",
  "started_at": "2026-04-17T21:00:00Z",
  "ready_repo_ids": ["keystone-hub"],
  "phase": "pre_promotion"
}`
	if err := os.WriteFile(filepath.Join(ctx.Config.StateDir, syncAttemptFileName), []byte(raw), 0o644); err != nil {
		t.Fatalf("write attempt artifact: %v", err)
	}
	manifest := Manifest{
		Schema:        "kstoolchain.adapter/v1alpha1",
		ManagedBinDir: ctx.Config.ManagedBinDir,
		Repos: []RepoAdapter{
			{
				RepoID:          "keystone-hub",
				RepoPath:        repoPath,
				ExpectedOutputs: []string{"kshub"},
				DirtyPolicy:     DirtyPolicyFailClosed,
				ReleaseUnit:     ReleaseUnitRepo,
				Status:          AdapterStatusReady,
			},
		},
	}
	persisted := PersistedState{
		Schema:        PersistedStateSchema,
		ManagedBinDir: ctx.Config.ManagedBinDir,
		Repos: []PersistedRepoState{
			{
				RepoID:                "keystone-hub",
				State:                 StateCurrent,
				RepoHead:              head,
				LastAttemptSourceKind: SourceKindCleanHead,
				ActiveBuild:           head,
				ActiveSourceKind:      SourceKindCleanHead,
			},
		},
	}

	report := BuildStatusReport(ctx, manifest, persisted, filepath.Join(ctx.Config.StateDir, "current.json"), true, false)
	if report.AttemptIntegrity == nil {
		t.Fatal("expected unknown attempt schema to fail closed")
	}
	if got := report.AttemptIntegrity.State; got != AttemptIntegrityArtifactBad {
		t.Fatalf("unexpected attempt integrity state: %s", got)
	}
}

func TestBuildStatusReportCurrentRequiresManagedPathResolution(t *testing.T) {
	home := t.TempDir()
	repoPath := testGitRepo(t, home)
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
				RepoID:          "keystone-hub",
				RepoPath:        repoPath,
				ExpectedOutputs: []string{"kshub"},
				DirtyPolicy:     DirtyPolicyFailClosed,
				ReleaseUnit:     ReleaseUnitRepo,
				Status:          AdapterStatusReady,
			},
		},
	}
	persisted := PersistedState{
		Schema:        PersistedStateSchema,
		ManagedBinDir: ctx.Config.ManagedBinDir,
		Repos: []PersistedRepoState{
			{RepoID: "keystone-hub", State: StateCurrent},
		},
	}

	report := BuildStatusReport(ctx, manifest, persisted, filepath.Join(ctx.Config.StateDir, "current.json"), true, false)
	if got := report.Repos[0].State; got != StateUnknown {
		t.Fatalf("unexpected repo state: %s", got)
	}
	if got := report.Repos[0].Outputs[0].State; got != StateUnknown {
		t.Fatalf("unexpected output state: %s", got)
	}
}

func TestBuildStatusReportSetupBlockedPreservesPersistedBuildTruth(t *testing.T) {
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
		Schema: "kstoolchain.adapter/v1alpha1",
		Repos: []RepoAdapter{
			{
				RepoID:          "keystone-hub",
				RepoPath:        "",
				ExpectedOutputs: []string{"kshub"},
				DirtyPolicy:     DirtyPolicyFailClosed,
				ReleaseUnit:     ReleaseUnitRepo,
				Status:          AdapterStatusReady,
			},
		},
	}
	persisted := PersistedState{
		Schema:        PersistedStateSchema,
		ManagedBinDir: ctx.Config.ManagedBinDir,
		Repos: []PersistedRepoState{
			{
				RepoID:      "keystone-hub",
				State:       StateCurrent,
				Reason:      "",
				RepoHead:    "deadbeef",
				ActiveBuild: "cafebabe",
			},
		},
	}

	report := BuildStatusReport(ctx, manifest, persisted, filepath.Join(ctx.Config.StateDir, "current.json"), true, false)
	if got := report.Repos[0].State; got != StateSetupBlocked {
		t.Fatalf("expected SETUP_BLOCKED, got %s", got)
	}
	if got := report.Repos[0].Reason; got != SetupReasonRepoPathUnset {
		t.Fatalf("unexpected setup reason: %s", got)
	}
	if report.Repos[0].RepoHead != "deadbeef" || report.Repos[0].ActiveBuild != "cafebabe" {
		t.Fatalf("expected persisted build truth to survive setup block, got %#v", report.Repos[0])
	}
	if got := report.Repos[0].Outputs[0].State; got != StateSetupBlocked {
		t.Fatalf("expected setup-blocked output, got %s", got)
	}
	if got := report.Summary.StateCounts[StateSetupBlocked]; got != 1 {
		t.Fatalf("expected setup-blocked count, got %d", got)
	}
	lines := RenderStatusText(report)
	foundNext := false
	for _, line := range lines {
		if strings.Contains(line, "run `kstoolchain init`") {
			foundNext = true
			break
		}
	}
	if !foundNext {
		t.Fatalf("expected setup-blocked status text to point back to init, got %#v", lines)
	}
}

func TestBuildStatusReportWarnsWhenSupportArtifactMissing(t *testing.T) {
	home := t.TempDir()
	repoPath := testGitRepo(t, home)
	activeBuild, ok := lookupRepoHead(repoPath)
	if !ok {
		t.Fatal("expected repo head")
	}
	ctx := &runtime.Context{
		HomeDir: home,
		Config: runtime.Config{
			ManagedBinDir: filepath.Join(home, ".keystone", "toolchain", "active", "bin"),
			StateDir:      filepath.Join(home, ".keystone", "toolchain", "state"),
		},
	}
	if err := os.MkdirAll(ctx.Config.ManagedBinDir, 0o755); err != nil {
		t.Fatalf("mkdir managed bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ctx.Config.ManagedBinDir, "ksctx"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write ksctx: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ctx.Config.ManagedBinDir, "ksctx-plugin-pg"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write ksctx-plugin-pg: %v", err)
	}
	t.Setenv("PATH", ctx.Config.ManagedBinDir)

	manifest := Manifest{
		Schema:        "kstoolchain.adapter/v1alpha1",
		ManagedBinDir: ctx.Config.ManagedBinDir,
		Repos: []RepoAdapter{
			{
				RepoID:           "keystone-context",
				RepoPath:         repoPath,
				ExpectedOutputs:  []string{"ksctx", "ksctx-plugin-pg"},
				SupportArtifacts: []string{".ksctx-runtime"},
				DirtyPolicy:      DirtyPolicyFailClosed,
				ReleaseUnit:      ReleaseUnitRepo,
				Status:           AdapterStatusReady,
			},
		},
	}
	persisted := PersistedState{
		Schema:        PersistedStateSchema,
		ManagedBinDir: ctx.Config.ManagedBinDir,
		Repos: []PersistedRepoState{
			{
				RepoID:      "keystone-context",
				State:       StateCurrent,
				ActiveBuild: activeBuild,
				Outputs:     []string{"ksctx", "ksctx-plugin-pg"},
			},
		},
	}

	report := BuildStatusReport(ctx, manifest, persisted, filepath.Join(ctx.Config.StateDir, "current.json"), true, false)
	if got := report.Repos[0].State; got != StateCurrent {
		t.Fatalf("missing support artifact should remain a warning, got state %s", got)
	}
	if len(report.Repos[0].Warnings) != 1 {
		t.Fatalf("expected one support warning, got %#v", report.Repos[0].Warnings)
	}
	if !strings.Contains(report.Repos[0].Warnings[0], ".ksctx-runtime") {
		t.Fatalf("unexpected support warning: %#v", report.Repos[0].Warnings)
	}
	if got := report.Summary.Overall; got != StateCurrent {
		t.Fatalf("support warning should not demote overall state, got %s", got)
	}
}

func TestLoadPersistedStateRejectsWrongSchema(t *testing.T) {
	home := t.TempDir()
	ctx := &runtime.Context{
		HomeDir: home,
		Config: runtime.Config{
			ManagedBinDir: filepath.Join(home, ".keystone", "toolchain", "active", "bin"),
			StateDir:      filepath.Join(home, ".keystone", "toolchain", "state"),
		},
	}
	if err := os.MkdirAll(ctx.Config.StateDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	stateFile := filepath.Join(ctx.Config.StateDir, "current.json")

	if err := os.WriteFile(stateFile, []byte(`{"schema":"wrong","managed_bin_dir":"`+ctx.Config.ManagedBinDir+`","repos":[]}`), 0o644); err != nil {
		t.Fatalf("write bad schema: %v", err)
	}
	if _, _, _, _, appErr := LoadPersistedState(ctx); appErr == nil {
		t.Fatal("expected bad schema error")
	}
}

func TestLoadPersistedStateRejectsLegacyV1Alpha1Schema(t *testing.T) {
	home := t.TempDir()
	ctx := &runtime.Context{
		HomeDir: home,
		Config: runtime.Config{
			ManagedBinDir: filepath.Join(home, ".keystone", "toolchain", "active", "bin"),
			StateDir:      filepath.Join(home, ".keystone", "toolchain", "state"),
		},
	}
	if err := os.MkdirAll(ctx.Config.StateDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	stateFile := filepath.Join(ctx.Config.StateDir, "current.json")
	body := `{"schema":"kstoolchain.state/v1alpha1","managed_bin_dir":"` + ctx.Config.ManagedBinDir + `","repos":[]}`
	if err := os.WriteFile(stateFile, []byte(body), 0o644); err != nil {
		t.Fatalf("write legacy schema: %v", err)
	}
	if _, _, _, _, appErr := LoadPersistedState(ctx); appErr == nil {
		t.Fatal("expected legacy schema rejection")
	}
}

func TestLoadPersistedStateRejectsMissingSourceKindInCurrentState(t *testing.T) {
	home := t.TempDir()
	ctx := &runtime.Context{
		HomeDir: home,
		Config: runtime.Config{
			ManagedBinDir: filepath.Join(home, ".keystone", "toolchain", "active", "bin"),
			StateDir:      filepath.Join(home, ".keystone", "toolchain", "state"),
		},
	}
	if err := os.MkdirAll(ctx.Config.StateDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	stateFile := filepath.Join(ctx.Config.StateDir, "current.json")
	body := `{"schema":"` + PersistedStateSchema + `","managed_bin_dir":"` + ctx.Config.ManagedBinDir + `","repos":[{"repo_id":"keystone-hub","state":"CURRENT","repo_head":"deadbeef","active_build":"deadbeef"}]}`
	if err := os.WriteFile(stateFile, []byte(body), 0o644); err != nil {
		t.Fatalf("write invalid source-kind state: %v", err)
	}
	if _, _, _, _, appErr := LoadPersistedState(ctx); appErr == nil {
		t.Fatal("expected missing source-kind rejection")
	}
}

func TestLoadPersistedStateManagedBinDirDriftReturnsContractDrift(t *testing.T) {
	home := t.TempDir()
	ctx := &runtime.Context{
		HomeDir: home,
		Config: runtime.Config{
			ManagedBinDir: filepath.Join(home, ".keystone", "toolchain", "active", "bin"),
			StateDir:      filepath.Join(home, ".keystone", "toolchain", "state"),
		},
	}
	if err := os.MkdirAll(ctx.Config.StateDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	stateFile := filepath.Join(ctx.Config.StateDir, "current.json")
	if err := os.WriteFile(stateFile, []byte(`{"schema":"`+PersistedStateSchema+`","managed_bin_dir":"`+filepath.Join(home, "other")+`","repos":[]}`), 0o644); err != nil {
		t.Fatalf("write drifted managed bin dir: %v", err)
	}
	persisted, gotStateFile, present, contractDrift, appErr := LoadPersistedState(ctx)
	if appErr != nil {
		t.Fatalf("unexpected app error: %v", appErr)
	}
	if !present {
		t.Fatal("expected present=true: state file exists even if config drifted")
	}
	if !contractDrift {
		t.Fatal("expected contractDrift=true for managed_bin_dir mismatch")
	}
	if gotStateFile != stateFile {
		t.Fatalf("unexpected state file path: %s", gotStateFile)
	}
	if len(persisted.Repos) != 0 {
		t.Fatalf("expected empty persisted state, got %#v", persisted)
	}
}

func TestValidatePersistedStateShapeRejectsMismatchedCurrentPairs(t *testing.T) {
	persisted := PersistedState{
		Schema:        PersistedStateSchema,
		ManagedBinDir: "/tmp/bin",
		Repos: []PersistedRepoState{
			{
				RepoID:                "keystone-hub",
				State:                 StateCurrent,
				RepoHead:              "deadbeef",
				LastAttemptSourceKind: SourceKindCleanHead,
				ActiveBuild:           "cafebabe",
				ActiveSourceKind:      SourceKindDirtyWorktree,
			},
		},
	}
	if appErr := validatePersistedStateShape(persisted); appErr == nil {
		t.Fatal("expected CURRENT pair mismatch rejection")
	}
}

func TestValidatePersistedStateShapeRejectsFailedStateWithActivePair(t *testing.T) {
	persisted := PersistedState{
		Schema:        PersistedStateSchema,
		ManagedBinDir: "/tmp/bin",
		Repos: []PersistedRepoState{
			{
				RepoID:                "keystone-hub",
				State:                 StateFailed,
				RepoHead:              "deadbeef",
				LastAttemptSourceKind: SourceKindCleanHead,
				ActiveBuild:           "deadbeef",
				ActiveSourceKind:      SourceKindCleanHead,
			},
		},
	}
	if appErr := validatePersistedStateShape(persisted); appErr == nil {
		t.Fatal("expected FAILED active-pair rejection")
	}
}

func TestStatusReportsContractDriftOnManagedBinMismatch(t *testing.T) {
	home := t.TempDir()
	repoPath := testGitRepo(t, home)
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
				RepoID:          "keystone-hub",
				RepoPath:        repoPath,
				ExpectedOutputs: []string{"kshub"},
				DirtyPolicy:     DirtyPolicyFailClosed,
				ReleaseUnit:     ReleaseUnitRepo,
				Status:          AdapterStatusReady,
			},
		},
	}
	// State file written for a different managed_bin_dir — simulates a user
	// who changed their managed_bin_dir config after a successful sync.
	report := BuildStatusReport(ctx, manifest, PersistedState{}, filepath.Join(ctx.Config.StateDir, "current.json"), true, true)
	if got := report.Summary.Overall; got != StateContractDrift {
		t.Fatalf("expected CONTRACT_DRIFT overall, got %s", got)
	}
	if got := report.Repos[0].State; got != StateContractDrift {
		t.Fatalf("expected CONTRACT_DRIFT repo state, got %s", got)
	}
	if report.Repos[0].Reason == "" {
		t.Fatal("expected non-empty reason for CONTRACT_DRIFT")
	}
}

func TestStatusHubReportsStaleWhenRepoHeadMoves(t *testing.T) {
	home := t.TempDir()
	hubRepo := filepath.Join(home, "hub")
	if err := os.MkdirAll(hubRepo, 0o755); err != nil {
		t.Fatalf("mkdir hub: %v", err)
	}
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = hubRepo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, string(out))
		}
	}
	runGit("init")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(hubRepo, "README.md"), []byte("first\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGit("add", "README.md")
	runGit("commit", "-m", "first")

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = hubRepo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
	activeBuild := strings.TrimSpace(string(out))

	// Advance HEAD past the active build.
	if err := os.WriteFile(filepath.Join(hubRepo, "README.md"), []byte("second\n"), 0o644); err != nil {
		t.Fatalf("write second: %v", err)
	}
	runGit("add", "README.md")
	runGit("commit", "-m", "second")
	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = hubRepo
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse second head: %v", err)
	}
	liveHead := strings.TrimSpace(string(out))

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
				RepoID:          "keystone-hub",
				RepoPath:        hubRepo,
				ExpectedOutputs: []string{"kshub"},
				DirtyPolicy:     DirtyPolicyFailClosed,
				ReleaseUnit:     ReleaseUnitRepo,
				Status:          AdapterStatusReady,
			},
		},
	}
	persisted := PersistedState{
		Schema:        PersistedStateSchema,
		ManagedBinDir: ctx.Config.ManagedBinDir,
		Repos: []PersistedRepoState{
			{
				RepoID:                "keystone-hub",
				State:                 StateCurrent,
				RepoHead:              activeBuild,
				LastAttemptSourceKind: SourceKindCleanHead,
				ActiveBuild:           activeBuild,
				ActiveSourceKind:      SourceKindCleanHead,
			},
		},
	}

	// Put a kshub binary in the managed bin dir so the output resolves correctly.
	// Without this, demoteMissingPathState downgrades STALE_LKG → UNKNOWN.
	// PATH only exposes the managed bin to prove repo HEAD lookup does not depend
	// on the ambient shell PATH still containing git.
	if err := os.MkdirAll(ctx.Config.ManagedBinDir, 0o755); err != nil {
		t.Fatalf("mkdir managed bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ctx.Config.ManagedBinDir, "kshub"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write kshub: %v", err)
	}
	t.Setenv("PATH", ctx.Config.ManagedBinDir)

	report := BuildStatusReport(ctx, manifest, persisted, filepath.Join(ctx.Config.StateDir, "current.json"), true, false)
	if got := report.Repos[0].State; got != StateStaleLKG {
		t.Fatalf("expected STALE_LKG when repo HEAD is ahead of active build, got %s", got)
	}
	if report.Repos[0].Reason == "" {
		t.Fatal("expected non-empty reason for STALE_LKG")
	}
	if got := report.Repos[0].RepoHead; got != activeBuild {
		t.Fatalf("expected repo_head to remain persisted classified-input truth, got %s want %s", got, activeBuild)
	}
	if report.Repos[0].RepoHead == liveHead {
		t.Fatalf("repo_head must not be overwritten with live observed HEAD %s", liveHead)
	}
}

func TestDeriveOverallRanksFAILEDAboveSHADOWED(t *testing.T) {
	counts := map[string]int{
		StateShadowed: 1,
		StateFailed:   1,
	}
	if got := deriveOverall(counts); got != StateFailed {
		t.Fatalf("expected FAILED to outrank SHADOWED, got %s", got)
	}
}

func TestDeriveOverallRanksSetupBlockedAboveShadowed(t *testing.T) {
	counts := map[string]int{
		StateShadowed:     1,
		StateSetupBlocked: 1,
	}
	if got := deriveOverall(counts); got != StateSetupBlocked {
		t.Fatalf("expected SETUP_BLOCKED to outrank SHADOWED, got %s", got)
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
				Warnings:      []string{"runtime bundle missing"},
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
	if !strings.Contains(joined, "keystone-memory  adapter=candidate  state=UNKNOWN") {
		t.Fatalf("unexpected text: %s", joined)
	}
	if !strings.Contains(joined, "warning: runtime bundle missing") {
		t.Fatalf("expected warnings in text render: %s", joined)
	}
	if strings.Contains(joined, "output ksmem") {
		t.Fatalf("non-ready adapter output section should be suppressed in text render: %s", joined)
	}
}

func TestBuildStatusReportCandidateNotShadowedWhenBinaryOnPath(t *testing.T) {
	home := t.TempDir()
	pathDir := filepath.Join(home, "path-bin")
	if err := os.MkdirAll(pathDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pathDir, "ksmem"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write ksmem: %v", err)
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
				RepoPath:        filepath.Join(home, "memory"),
				ExpectedOutputs: []string{"ksmem"},
				DirtyPolicy:     DirtyPolicyFailClosed,
				ReleaseUnit:     ReleaseUnitRepo,
				Status:          AdapterStatusCandidate,
			},
		},
	}

	report := BuildStatusReport(ctx, manifest, PersistedState{}, filepath.Join(ctx.Config.StateDir, "current.json"), false, false)
	if got := report.Repos[0].State; got != StateUnknown {
		t.Fatalf("candidate adapter with binary on PATH should be UNKNOWN not %s", got)
	}
	if report.Repos[0].Outputs[0].ResolvedPath != "" {
		t.Fatalf("candidate adapter output should have no resolved_path")
	}
	if report.Summary.OutputCount != 0 {
		t.Fatalf("managed output count should be 0 for candidate-only suite, got %d", report.Summary.OutputCount)
	}
}

func TestBuildStatusReportDemotedAdapterDoesNotShowPersistedState(t *testing.T) {
	// If an adapter previously had state=CURRENT in persisted state but has since
	// been demoted to candidate, it must show UNKNOWN, not CURRENT.
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
				RepoPath:        filepath.Join(home, "memory"),
				ExpectedOutputs: []string{"ksmem"},
				DirtyPolicy:     DirtyPolicyFailClosed,
				ReleaseUnit:     ReleaseUnitRepo,
				Status:          AdapterStatusCandidate, // demoted
			},
		},
	}
	persisted := PersistedState{
		Schema:        PersistedStateSchema,
		ManagedBinDir: ctx.Config.ManagedBinDir,
		Repos: []PersistedRepoState{
			{RepoID: "keystone-memory", State: StateCurrent}, // stale
		},
	}

	report := BuildStatusReport(ctx, manifest, persisted, filepath.Join(ctx.Config.StateDir, "current.json"), true, false)
	if got := report.Repos[0].State; got != StateUnknown {
		t.Fatalf("demoted adapter with stale CURRENT persisted state should be UNKNOWN, got %s", got)
	}
}
