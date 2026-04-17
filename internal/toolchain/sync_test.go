package toolchain

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/kanini/keystone-toolchain/internal/runtime"
)

func TestSyncDoesNotPromoteWhenProbeFails(t *testing.T) {
	home := t.TempDir()
	ctx := testContext(home)
	repoPath := testGitRepo(t, home)
	installScript := filepath.Join(home, "install.sh")
	writeExecutable(t, installScript, "#!/bin/sh\nset -eu\nmkdir -p \"$1\"\nprintf '#!/bin/sh\\necho ok\\n' > \"$1/kshub\"\nchmod +x \"$1/kshub\"\n")
	manifest := Manifest{
		Schema:        "kstoolchain.adapter/v1alpha1",
		ManagedBinDir: ctx.Config.ManagedBinDir,
		Repos: []RepoAdapter{
			{
				RepoID:          "keystone-hub",
				RepoPath:        repoPath,
				InstallCmd:      []string{installScript, "{{stage_bin}}"},
				ExpectedOutputs: []string{"kshub"},
				ProbeCmd:        []string{"/bin/sh", "-c", "exit 9"},
				DirtyPolicy:     DirtyPolicyFailClosed,
				ReleaseUnit:     ReleaseUnitRepo,
				Status:          AdapterStatusReady,
			},
		},
	}

	if appErr := SyncReadySet(ctx, manifest, PersistedState{}, false, SyncOptions{}); appErr != nil {
		t.Fatalf("unexpected app error: %v", appErr)
	}
	persisted, _, _, _, appErr := LoadPersistedState(ctx)
	if appErr != nil {
		t.Fatalf("load state: %v", appErr)
	}
	if got := persisted.Repos[0].State; got != StateFailed {
		t.Fatalf("unexpected state: %s", got)
	}
	if _, err := os.Stat(filepath.Join(ctx.Config.ManagedBinDir, "kshub")); !os.IsNotExist(err) {
		t.Fatalf("expected no promoted binary, got err=%v", err)
	}
}

func TestSyncWritesStateAndStatusReadsIt(t *testing.T) {
	home := t.TempDir()
	ctx := testContext(home)
	repoPath := testGitRepo(t, home)
	installScript := filepath.Join(home, "install.sh")
	writeExecutable(t, installScript, "#!/bin/sh\nset -eu\nmkdir -p \"$1\"\nprintf '#!/bin/sh\\necho kshub test\\n' > \"$1/kshub\"\nchmod +x \"$1/kshub\"\n")
	manifest := Manifest{
		Schema:        "kstoolchain.adapter/v1alpha1",
		ManagedBinDir: ctx.Config.ManagedBinDir,
		Repos: []RepoAdapter{
			{
				RepoID:          "keystone-hub",
				RepoPath:        repoPath,
				InstallCmd:      []string{installScript, "{{stage_bin}}"},
				ExpectedOutputs: []string{"kshub"},
				ProbeCmd:        []string{"/bin/sh", "-c", "\"{{stage_bin}}/kshub\" >/dev/null"},
				DirtyPolicy:     DirtyPolicyFailClosed,
				ReleaseUnit:     ReleaseUnitRepo,
				Status:          AdapterStatusReady,
			},
		},
	}

	if appErr := SyncReadySet(ctx, manifest, PersistedState{}, false, SyncOptions{}); appErr != nil {
		t.Fatalf("unexpected app error: %v", appErr)
	}
	persisted, stateFile, present, _, appErr := LoadPersistedState(ctx)
	if appErr != nil {
		t.Fatalf("load state: %v", appErr)
	}
	if !present {
		t.Fatal("expected state file to exist")
	}
	t.Setenv("PATH", "")
	report := BuildStatusReport(ctx, manifest, persisted, stateFile, true, false)
	if got := report.Repos[0].State; got != StateUnknown {
		t.Fatalf("expected repo state UNKNOWN until PATH points at managed bin, got %s", got)
	}

	t.Setenv("PATH", ctx.Config.ManagedBinDir)
	report = BuildStatusReport(ctx, manifest, persisted, stateFile, true, false)
	if got := report.Repos[0].State; got != StateCurrent {
		t.Fatalf("unexpected repo state after PATH update: %s", got)
	}
	if persisted.Repos[0].ActiveBuild == "" {
		t.Fatal("expected active build")
	}
}

func TestSyncDirtyRepoFailsClosed(t *testing.T) {
	home := t.TempDir()
	ctx := testContext(home)
	repoPath := testGitRepo(t, home)
	if err := os.WriteFile(filepath.Join(repoPath, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}
	manifest := Manifest{
		Schema:        "kstoolchain.adapter/v1alpha1",
		ManagedBinDir: ctx.Config.ManagedBinDir,
		Repos: []RepoAdapter{
			{
				RepoID:          "keystone-hub",
				RepoPath:        repoPath,
				InstallCmd:      []string{"/bin/sh", "-c", "exit 0"},
				ExpectedOutputs: []string{"kshub"},
				ProbeCmd:        []string{"/bin/sh", "-c", "exit 0"},
				DirtyPolicy:     DirtyPolicyFailClosed,
				ReleaseUnit:     ReleaseUnitRepo,
				Status:          AdapterStatusReady,
			},
		},
	}

	if appErr := SyncReadySet(ctx, manifest, PersistedState{}, false, SyncOptions{}); appErr != nil {
		t.Fatalf("unexpected app error: %v", appErr)
	}
	persisted, _, _, _, appErr := LoadPersistedState(ctx)
	if appErr != nil {
		t.Fatalf("load state: %v", appErr)
	}
	if got := persisted.Repos[0].State; got != StateDirtySkipped {
		t.Fatalf("unexpected state: %s", got)
	}
	if !strings.Contains(persisted.Repos[0].Reason, "fail_closed") {
		t.Fatalf("unexpected reason: %q", persisted.Repos[0].Reason)
	}
	if got := persisted.Repos[0].LastAttemptSourceKind; got != SourceKindDirtyWorktree {
		t.Fatalf("unexpected last_attempt_source_kind: %s", got)
	}
	if strings.TrimSpace(persisted.Repos[0].RepoHead) == "" {
		t.Fatal("expected dirty skip to persist repo_head")
	}
	if got := persisted.Repos[0].ActiveSourceKind; got != "" {
		t.Fatalf("dirty skip without prior active build must not invent active_source_kind, got %q", got)
	}
}

func TestSyncAllowDirtyPromotesCurrentWithDirtyProvenance(t *testing.T) {
	home := t.TempDir()
	ctx := testContext(home)
	repoPath := testGitRepo(t, home)
	if err := os.WriteFile(filepath.Join(repoPath, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}
	installScript := filepath.Join(home, "install.sh")
	writeExecutable(t, installScript, "#!/bin/sh\nset -eu\nmkdir -p \"$1\"\nprintf '#!/bin/sh\\necho kshub test\\n' > \"$1/kshub\"\nchmod +x \"$1/kshub\"\n")
	manifest := Manifest{
		Schema:        "kstoolchain.adapter/v1alpha1",
		ManagedBinDir: ctx.Config.ManagedBinDir,
		Repos: []RepoAdapter{
			{
				RepoID:          "keystone-hub",
				RepoPath:        repoPath,
				InstallCmd:      []string{installScript, "{{stage_bin}}"},
				ExpectedOutputs: []string{"kshub"},
				ProbeCmd:        []string{"/bin/sh", "-c", "\"{{stage_bin}}/kshub\" >/dev/null"},
				DirtyPolicy:     DirtyPolicyFailClosed,
				ReleaseUnit:     ReleaseUnitRepo,
				Status:          AdapterStatusReady,
			},
		},
	}

	if appErr := SyncReadySet(ctx, manifest, PersistedState{}, false, SyncOptions{AllowDirty: true}); appErr != nil {
		t.Fatalf("unexpected app error: %v", appErr)
	}
	persisted, _, _, _, appErr := LoadPersistedState(ctx)
	if appErr != nil {
		t.Fatalf("load state: %v", appErr)
	}
	repo := persisted.Repos[0]
	if got := repo.State; got != StateCurrent {
		t.Fatalf("unexpected state: %s", got)
	}
	if repo.RepoHead == "" || repo.ActiveBuild == "" {
		t.Fatalf("expected both provenance heads, got %#v", repo)
	}
	if repo.RepoHead != repo.ActiveBuild {
		t.Fatalf("expected matching classified and active heads, got %#v", repo)
	}
	if repo.LastAttemptSourceKind != SourceKindDirtyWorktree || repo.ActiveSourceKind != SourceKindDirtyWorktree {
		t.Fatalf("expected dirty provenance on both pairs, got %#v", repo)
	}
}

func TestSyncAllowDirtyFailurePreservesPriorActivePair(t *testing.T) {
	home := t.TempDir()
	ctx := testContext(home)
	repoPath := testGitRepo(t, home)
	if err := os.WriteFile(filepath.Join(repoPath, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}
	manifest := Manifest{
		Schema:        "kstoolchain.adapter/v1alpha1",
		ManagedBinDir: ctx.Config.ManagedBinDir,
		Repos: []RepoAdapter{
			{
				RepoID:          "keystone-hub",
				RepoPath:        repoPath,
				InstallCmd:      []string{"/bin/sh", "-c", "exit 23"},
				ExpectedOutputs: []string{"kshub"},
				ProbeCmd:        []string{"/bin/sh", "-c", "exit 0"},
				DirtyPolicy:     DirtyPolicyFailClosed,
				ReleaseUnit:     ReleaseUnitRepo,
				Status:          AdapterStatusReady,
			},
		},
	}
	prior := PersistedState{
		Schema:        PersistedStateSchema,
		ManagedBinDir: ctx.Config.ManagedBinDir,
		Repos: []PersistedRepoState{
			{
				RepoID:                "keystone-hub",
				State:                 StateCurrent,
				RepoHead:              "deadbeef",
				LastAttemptSourceKind: SourceKindCleanHead,
				ActiveBuild:           "deadbeef",
				ActiveSourceKind:      SourceKindCleanHead,
			},
		},
	}

	if appErr := SyncReadySet(ctx, manifest, prior, true, SyncOptions{AllowDirty: true}); appErr != nil {
		t.Fatalf("unexpected app error: %v", appErr)
	}
	persisted, _, _, _, appErr := LoadPersistedState(ctx)
	if appErr != nil {
		t.Fatalf("load state: %v", appErr)
	}
	repo := persisted.Repos[0]
	if got := repo.State; got != StateStaleLKG {
		t.Fatalf("unexpected state: %s", got)
	}
	if repo.ActiveBuild != "deadbeef" || repo.ActiveSourceKind != SourceKindCleanHead {
		t.Fatalf("expected prior active pair to survive dirty failure, got %#v", repo)
	}
	if repo.LastAttemptSourceKind != SourceKindDirtyWorktree || repo.RepoHead == "" {
		t.Fatalf("expected dirty classified-input pair on failed dirty override, got %#v", repo)
	}
}

func TestSyncClassificationFailurePreservesPriorClassifiedPair(t *testing.T) {
	home := t.TempDir()
	ctx := testContext(home)
	repoPath := filepath.Join(home, "repo")
	if err := os.MkdirAll(filepath.Join(repoPath, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir fake repo: %v", err)
	}
	gitBin := filepath.Join(home, "fake-bin", "git")
	if err := os.MkdirAll(filepath.Dir(gitBin), 0o755); err != nil {
		t.Fatalf("mkdir fake git dir: %v", err)
	}
	writeExecutable(t, gitBin, "#!/bin/sh\nset -eu\ncmd=${3:-}\nif [ \"$cmd\" = \"rev-parse\" ] && [ \"${4:-}\" = \"HEAD\" ]; then\n  printf 'deadbeef\\n'\n  exit 0\nfi\nif [ \"$cmd\" = \"status\" ]; then\n  echo 'boom' >&2\n  exit 17\nfi\nexit 19\n")
	t.Setenv("PATH", filepath.Dir(gitBin))

	manifest := Manifest{
		Schema: "kstoolchain.adapter/v1alpha1",
		Repos: []RepoAdapter{
			{
				RepoID:          "keystone-hub",
				RepoPath:        repoPath,
				InstallCmd:      []string{"/bin/sh", "-c", "exit 0"},
				ExpectedOutputs: []string{"kshub"},
				ProbeCmd:        []string{"/bin/sh", "-c", "exit 0"},
				DirtyPolicy:     DirtyPolicyFailClosed,
				ReleaseUnit:     ReleaseUnitRepo,
				Status:          AdapterStatusReady,
			},
		},
	}
	prior := PersistedState{
		Schema:        PersistedStateSchema,
		ManagedBinDir: ctx.Config.ManagedBinDir,
		Repos: []PersistedRepoState{
			{
				RepoID:                "keystone-hub",
				State:                 StateCurrent,
				RepoHead:              "deadbeef",
				LastAttemptSourceKind: SourceKindCleanHead,
				ActiveBuild:           "deadbeef",
				ActiveSourceKind:      SourceKindCleanHead,
			},
		},
	}

	if appErr := SyncReadySet(ctx, manifest, prior, true, SyncOptions{}); appErr != nil {
		t.Fatalf("unexpected app error: %v", appErr)
	}
	persisted, _, _, _, appErr := LoadPersistedState(ctx)
	if appErr != nil {
		t.Fatalf("load state: %v", appErr)
	}
	repo := persisted.Repos[0]
	if got := repo.State; got != StateStaleLKG {
		t.Fatalf("unexpected state: %s", got)
	}
	if repo.RepoHead != "deadbeef" || repo.LastAttemptSourceKind != SourceKindCleanHead {
		t.Fatalf("expected prior classified pair to survive classification failure, got %#v", repo)
	}
	if repo.ActiveBuild != "deadbeef" || repo.ActiveSourceKind != SourceKindCleanHead {
		t.Fatalf("expected prior active pair to survive classification failure, got %#v", repo)
	}
	if !strings.Contains(repo.Reason, "could not inspect repo dirtiness") {
		t.Fatalf("unexpected reason: %q", repo.Reason)
	}
}

func TestSyncPromotionBoundaryRevalidationFailsClosed(t *testing.T) {
	home := t.TempDir()
	ctx := testContext(home)
	repoPath := testGitRepo(t, home)
	installScript := filepath.Join(home, "install-dirty-before-promote.sh")
	writeExecutable(t, installScript, fmt.Sprintf("#!/bin/sh\nset -eu\nmkdir -p \"$1\"\nprintf '#!/bin/sh\\necho ok\\n' > \"$1/kshub\"\nchmod +x \"$1/kshub\"\nprintf 'dirty\\n' > %q\n", filepath.Join(repoPath, "dirty.txt")))
	manifest := Manifest{
		Schema:        "kstoolchain.adapter/v1alpha1",
		ManagedBinDir: ctx.Config.ManagedBinDir,
		Repos: []RepoAdapter{
			{
				RepoID:          "keystone-hub",
				RepoPath:        repoPath,
				InstallCmd:      []string{installScript, "{{stage_bin}}"},
				ExpectedOutputs: []string{"kshub"},
				ProbeCmd:        []string{"/bin/sh", "-c", "\"{{stage_bin}}/kshub\" >/dev/null"},
				DirtyPolicy:     DirtyPolicyFailClosed,
				ReleaseUnit:     ReleaseUnitRepo,
				Status:          AdapterStatusReady,
			},
		},
	}

	if appErr := SyncReadySet(ctx, manifest, PersistedState{}, false, SyncOptions{}); appErr != nil {
		t.Fatalf("unexpected app error: %v", appErr)
	}
	persisted, _, _, _, appErr := LoadPersistedState(ctx)
	if appErr != nil {
		t.Fatalf("load state: %v", appErr)
	}
	repo := persisted.Repos[0]
	if got := repo.State; got != StateFailed {
		t.Fatalf("unexpected state: %s", got)
	}
	if repo.ActiveBuild != "" || repo.ActiveSourceKind != "" {
		t.Fatalf("promotion-boundary failure must not persist a new active pair, got %#v", repo)
	}
	if repo.RepoHead == "" || repo.LastAttemptSourceKind != SourceKindCleanHead {
		t.Fatalf("expected trustworthy clean classified-input pair to survive, got %#v", repo)
	}
	if !strings.Contains(repo.Reason, "promotion boundary") && !strings.Contains(repo.Reason, "became dirty") {
		t.Fatalf("unexpected reason: %q", repo.Reason)
	}
	if _, err := os.Stat(filepath.Join(ctx.Config.ManagedBinDir, "kshub")); !os.IsNotExist(err) {
		t.Fatalf("expected no promoted binary on promotion-boundary failure, got err=%v", err)
	}
}

func TestPromoteFileFallsBackAcrossFilesystems(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	source := filepath.Join(sourceDir, "kshub")
	target := filepath.Join(targetDir, "kshub")
	writeExecutable(t, source, "#!/bin/sh\necho promoted\n")

	calls := 0
	renameFn := func(oldPath, newPath string) error {
		calls++
		if calls == 1 {
			return &os.LinkError{Op: "rename", Old: oldPath, New: newPath, Err: syscall.EXDEV}
		}
		return os.Rename(oldPath, newPath)
	}

	if err := promotePath(source, target, renameFn); err != nil {
		t.Fatalf("promotePath: %v", err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("expected source to be removed, got err=%v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if !strings.Contains(string(data), "promoted") {
		t.Fatalf("unexpected target contents: %q", string(data))
	}
}

func TestPromoteDirectoryFallsBackAcrossFilesystems(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	source := filepath.Join(sourceRoot, ".ksctx-runtime")
	target := filepath.Join(targetRoot, ".ksctx-runtime")
	if err := os.MkdirAll(filepath.Join(source, "mcp"), 0o755); err != nil {
		t.Fatalf("mkdir source runtime: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "ksctx.js"), []byte("runtime\n"), 0o644); err != nil {
		t.Fatalf("write runtime js: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "mcp", "schema.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write runtime schema: %v", err)
	}

	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir existing target runtime: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "stale.txt"), []byte("old\n"), 0o644); err != nil {
		t.Fatalf("write stale runtime file: %v", err)
	}

	calls := 0
	renameFn := func(oldPath, newPath string) error {
		calls++
		if calls == 1 {
			return &os.LinkError{Op: "rename", Old: oldPath, New: newPath, Err: syscall.EXDEV}
		}
		return os.Rename(oldPath, newPath)
	}

	if err := promotePath(source, target, renameFn); err != nil {
		t.Fatalf("promotePath directory: %v", err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("expected source runtime dir to be removed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "stale.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected stale target contents to be replaced, got err=%v", err)
	}
	if data, err := os.ReadFile(filepath.Join(target, "ksctx.js")); err != nil {
		t.Fatalf("read promoted runtime js: %v", err)
	} else if !strings.Contains(string(data), "runtime") {
		t.Fatalf("unexpected runtime js contents: %q", string(data))
	}
	if _, err := os.Stat(filepath.Join(target, "mcp", "schema.json")); err != nil {
		t.Fatalf("expected nested runtime artifact: %v", err)
	}
}

func TestSyncPromotesSupportArtifactsForKeystoneContext(t *testing.T) {
	home := t.TempDir()
	ctx := testContext(home)
	repoPath := testGitRepo(t, home)
	installScript := filepath.Join(home, "install-ksctx.sh")
	writeExecutable(t, installScript, "#!/bin/sh\nset -eu\nmkdir -p \"$1/.ksctx-runtime/mcp\"\nprintf '#!/bin/sh\\necho ksctx\\n' > \"$1/ksctx\"\nprintf '#!/bin/sh\\necho plugin\\n' > \"$1/ksctx-plugin-pg\"\nprintf 'console.log(1)\\n' > \"$1/.ksctx-runtime/ksctx.js\"\nprintf '{}\\n' > \"$1/.ksctx-runtime/mcp/schema.json\"\nchmod +x \"$1/ksctx\" \"$1/ksctx-plugin-pg\"\n")
	manifest := Manifest{
		Schema:        "kstoolchain.adapter/v1alpha1",
		ManagedBinDir: ctx.Config.ManagedBinDir,
		Repos: []RepoAdapter{
			{
				RepoID:           "keystone-context",
				RepoPath:         repoPath,
				InstallCmd:       []string{installScript, "{{stage_bin}}"},
				ExpectedOutputs:  []string{"ksctx", "ksctx-plugin-pg"},
				SupportArtifacts: []string{".ksctx-runtime"},
				ProbeCmd:         []string{"/bin/sh", "-c", "\"{{stage_bin}}/ksctx\" >/dev/null"},
				DirtyPolicy:      DirtyPolicyFailClosed,
				ReleaseUnit:      ReleaseUnitRepo,
				Status:           AdapterStatusReady,
			},
		},
	}

	if appErr := SyncReadySet(ctx, manifest, PersistedState{}, false, SyncOptions{}); appErr != nil {
		t.Fatalf("unexpected app error: %v", appErr)
	}
	if _, err := os.Stat(filepath.Join(ctx.Config.ManagedBinDir, "ksctx")); err != nil {
		t.Fatalf("expected promoted ksctx launcher: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ctx.Config.ManagedBinDir, "ksctx-plugin-pg")); err != nil {
		t.Fatalf("expected promoted ksctx plugin launcher: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ctx.Config.ManagedBinDir, ".ksctx-runtime", "ksctx.js")); err != nil {
		t.Fatalf("expected promoted runtime js: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ctx.Config.ManagedBinDir, ".ksctx-runtime", "mcp", "schema.json")); err != nil {
		t.Fatalf("expected promoted runtime schema: %v", err)
	}

	writeExecutable(t, installScript, "#!/bin/sh\nset -eu\nmkdir -p \"$1/.ksctx-runtime/mcp\"\nprintf '#!/bin/sh\\necho ksctx-v2\\n' > \"$1/ksctx\"\nprintf '#!/bin/sh\\necho plugin-v2\\n' > \"$1/ksctx-plugin-pg\"\nprintf 'console.log(2)\\n' > \"$1/.ksctx-runtime/ksctx.js\"\nprintf '{\"v\":2}\\n' > \"$1/.ksctx-runtime/mcp/schema.json\"\nchmod +x \"$1/ksctx\" \"$1/ksctx-plugin-pg\"\n")
	if appErr := SyncReadySet(ctx, manifest, PersistedState{}, false, SyncOptions{}); appErr != nil {
		t.Fatalf("unexpected app error on second sync: %v", appErr)
	}
	if data, err := os.ReadFile(filepath.Join(ctx.Config.ManagedBinDir, ".ksctx-runtime", "ksctx.js")); err != nil {
		t.Fatalf("read promoted runtime js after second sync: %v", err)
	} else if !strings.Contains(string(data), "console.log(2)") {
		t.Fatalf("expected second sync to replace runtime js, got %q", string(data))
	}
	if data, err := os.ReadFile(filepath.Join(ctx.Config.ManagedBinDir, ".ksctx-runtime", "mcp", "schema.json")); err != nil {
		t.Fatalf("read promoted runtime schema after second sync: %v", err)
	} else if !strings.Contains(string(data), "\"v\":2") {
		t.Fatalf("expected second sync to replace runtime schema, got %q", string(data))
	}
}

func TestSyncPreflightsSupportArtifactsBeforePromotion(t *testing.T) {
	home := t.TempDir()
	ctx := testContext(home)
	repoPath := testGitRepo(t, home)
	installScript := filepath.Join(home, "install-missing-runtime.sh")
	writeExecutable(t, installScript, "#!/bin/sh\nset -eu\nprintf '#!/bin/sh\\necho ksctx\\n' > \"$1/ksctx\"\nprintf '#!/bin/sh\\necho plugin\\n' > \"$1/ksctx-plugin-pg\"\nchmod +x \"$1/ksctx\" \"$1/ksctx-plugin-pg\"\n")
	manifest := Manifest{
		Schema:        "kstoolchain.adapter/v1alpha1",
		ManagedBinDir: ctx.Config.ManagedBinDir,
		Repos: []RepoAdapter{
			{
				RepoID:           "keystone-context",
				RepoPath:         repoPath,
				InstallCmd:       []string{installScript, "{{stage_bin}}"},
				ExpectedOutputs:  []string{"ksctx", "ksctx-plugin-pg"},
				SupportArtifacts: []string{".ksctx-runtime"},
				ProbeCmd:         []string{"/bin/sh", "-c", "\"{{stage_bin}}/ksctx\" >/dev/null"},
				DirtyPolicy:      DirtyPolicyFailClosed,
				ReleaseUnit:      ReleaseUnitRepo,
				Status:           AdapterStatusReady,
			},
		},
	}

	if appErr := SyncReadySet(ctx, manifest, PersistedState{}, false, SyncOptions{}); appErr != nil {
		t.Fatalf("unexpected app error: %v", appErr)
	}
	persisted, _, _, _, appErr := LoadPersistedState(ctx)
	if appErr != nil {
		t.Fatalf("load state: %v", appErr)
	}
	if got := persisted.Repos[0].State; got != StateFailed {
		t.Fatalf("unexpected repo state: %s", got)
	}
	if !strings.Contains(persisted.Repos[0].Reason, "expected staged artifact .ksctx-runtime is missing") {
		t.Fatalf("unexpected failure reason: %q", persisted.Repos[0].Reason)
	}
	if _, err := os.Stat(filepath.Join(ctx.Config.ManagedBinDir, "ksctx")); !os.IsNotExist(err) {
		t.Fatalf("expected no promoted launcher when support artifact is missing, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(ctx.Config.ManagedBinDir, ".ksctx-runtime")); !os.IsNotExist(err) {
		t.Fatalf("expected no promoted runtime dir when support artifact is missing, got err=%v", err)
	}
}

func TestSyncMarksReadyAdapterSetupBlockedBeforeRepoWork(t *testing.T) {
	home := t.TempDir()
	ctx := testContext(home)
	manifest := Manifest{
		Schema: "kstoolchain.adapter/v1alpha1",
		Repos: []RepoAdapter{
			{
				RepoID:          "keystone-hub",
				RepoPath:        "",
				InstallCmd:      []string{"/bin/sh", "-c", "exit 99"},
				ExpectedOutputs: []string{"kshub"},
				ProbeCmd:        []string{"/bin/sh", "-c", "exit 99"},
				DirtyPolicy:     DirtyPolicyFailClosed,
				ReleaseUnit:     ReleaseUnitRepo,
				Status:          AdapterStatusReady,
			},
		},
	}
	prior := PersistedState{
		Schema:        PersistedStateSchema,
		ManagedBinDir: ctx.Config.ManagedBinDir,
		LastSuccessAt: "2026-04-01T00:00:00Z",
		Repos: []PersistedRepoState{
			{
				RepoID:                "keystone-hub",
				State:                 StateCurrent,
				Reason:                "",
				RepoHead:              "deadbeef",
				LastAttemptSourceKind: SourceKindCleanHead,
				ActiveBuild:           "deadbeef",
				ActiveSourceKind:      SourceKindCleanHead,
			},
		},
	}

	if appErr := SyncReadySet(ctx, manifest, prior, true, SyncOptions{}); appErr != nil {
		t.Fatalf("unexpected app error: %v", appErr)
	}
	persisted, _, _, _, appErr := LoadPersistedState(ctx)
	if appErr != nil {
		t.Fatalf("load state: %v", appErr)
	}
	if got := persisted.Repos[0].State; got != StateSetupBlocked {
		t.Fatalf("unexpected state: %s", got)
	}
	if got := persisted.Repos[0].Reason; got != SetupReasonRepoPathUnset {
		t.Fatalf("unexpected setup reason: %s", got)
	}
	if persisted.Repos[0].RepoHead != "deadbeef" || persisted.Repos[0].ActiveBuild != "deadbeef" {
		t.Fatalf("expected prior build truth to survive setup block, got %#v", persisted.Repos[0])
	}
	if persisted.Repos[0].LastAttemptSourceKind != SourceKindCleanHead || persisted.Repos[0].ActiveSourceKind != SourceKindCleanHead {
		t.Fatalf("expected setup block to preserve prior source kinds, got %#v", persisted.Repos[0])
	}
	if persisted.LastSuccessAt != prior.LastSuccessAt {
		t.Fatalf("expected LastSuccessAt to remain unchanged, got %s", persisted.LastSuccessAt)
	}
}

func testContext(home string) *runtime.Context {
	return &runtime.Context{
		HomeDir: home,
		Config: runtime.Config{
			ManagedBinDir: filepath.Join(home, ".keystone", "toolchain", "active", "bin"),
			StateDir:      filepath.Join(home, ".keystone", "toolchain", "state"),
		},
		Now: func() time.Time {
			return time.Date(2026, 4, 13, 4, 7, 0, 0, time.UTC)
		},
	}
}

func testGitRepo(t *testing.T, home string) string {
	t.Helper()
	repoPath := filepath.Join(home, "repo")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	runCmd(t, repoPath, "git", "init")
	runCmd(t, repoPath, "git", "config", "user.email", "test@example.com")
	runCmd(t, repoPath, "git", "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("test\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runCmd(t, repoPath, "git", "add", "README.md")
	runCmd(t, repoPath, "git", "commit", "-m", "init")
	return repoPath
}

func writeExecutable(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
}

func runCmd(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, string(out))
	}
}
