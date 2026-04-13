package toolchain

import (
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

	if appErr := SyncReadySet(ctx, manifest, PersistedState{}, false); appErr != nil {
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

	if appErr := SyncReadySet(ctx, manifest, PersistedState{}, false); appErr != nil {
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

	if appErr := SyncReadySet(ctx, manifest, PersistedState{}, false); appErr != nil {
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

	if err := promoteFile(source, target, renameFn); err != nil {
		t.Fatalf("promoteFile: %v", err)
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
