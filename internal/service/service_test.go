package service

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kanini/keystone-toolchain/internal/contract"
	"github.com/kanini/keystone-toolchain/internal/runtime"
	"github.com/kanini/keystone-toolchain/internal/toolchain"
)

func TestInitReportDelegatesThroughSharedReadySetExecutor(t *testing.T) {
	home := t.TempDir()
	testGitRepo(t, filepath.Join(home, "git", "keystone-hub"))
	t.Setenv("SHELL", "/bin/bash")

	svc := New(testServiceContext(home))
	delegateCalls := 0
	svc.readySetExecutor = func(toolchain.SyncOptions) (toolchain.SyncReport, []contract.Warning, int, *contract.AppError) {
		delegateCalls++
		report := toolchain.SyncReport{
			Schema:  toolchain.SyncReportSchema,
			Outcome: toolchain.SyncOutcomeSucceeded,
			Summary: toolchain.SyncSummary{
				ReadyRepoCount:   1,
				UpdatedRepoCount: 1,
			},
			PrimaryNextAction: "open a new shell or source your rc file so /tmp/bin wins on PATH",
			FinalStatus: toolchain.StatusReport{
				Schema:        toolchain.StatusReportSchema,
				ManagedBinDir: svc.ctx.Config.ManagedBinDir,
				Summary: toolchain.StatusSummary{
					Overall:     toolchain.StateCurrent,
					StateCounts: map[string]int{toolchain.StateCurrent: 1},
				},
			},
		}
		return report, nil, report.ExitCode(), nil
	}

	var stdout bytes.Buffer
	report, warnings, exitCode, appErr := svc.InitReport(strings.NewReader("\ny\n"), &stdout, toolchain.InitOptions{})
	if appErr != nil {
		t.Fatalf("init report: %v", appErr)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
	if exitCode != contract.ExitOK {
		t.Fatalf("unexpected exit code: got=%d want=%d", exitCode, contract.ExitOK)
	}
	if delegateCalls != 1 {
		t.Fatalf("expected exactly one shared ready-set call, got %d", delegateCalls)
	}
	if !report.Delegated || report.ReadySet == nil {
		t.Fatalf("expected delegated ready-set report, got %#v", report)
	}
	if _, err := os.Stat(filepath.Join(home, ".keystone", "toolchain", "state", "current.json")); !os.IsNotExist(err) {
		t.Fatalf("expected init not to write current.json outside shared sync path, got err=%v", err)
	}
}

func TestInitReportDryRunDoesNotDelegateOrWriteState(t *testing.T) {
	home := t.TempDir()
	testGitRepo(t, filepath.Join(home, "git", "keystone-hub"))
	t.Setenv("SHELL", "/bin/bash")

	svc := New(testServiceContext(home))
	delegateCalls := 0
	svc.readySetExecutor = func(toolchain.SyncOptions) (toolchain.SyncReport, []contract.Warning, int, *contract.AppError) {
		delegateCalls++
		return toolchain.SyncReport{}, nil, contract.ExitOK, nil
	}

	var stdout bytes.Buffer
	report, _, exitCode, appErr := svc.InitReport(strings.NewReader(""), &stdout, toolchain.InitOptions{DryRun: true})
	if appErr != nil {
		t.Fatalf("init dry-run: %v", appErr)
	}
	if exitCode != contract.ExitOK {
		t.Fatalf("unexpected exit code: got=%d want=%d", exitCode, contract.ExitOK)
	}
	if report.Delegated {
		t.Fatalf("dry-run must not delegate, got %#v", report)
	}
	if delegateCalls != 0 {
		t.Fatalf("dry-run must not call shared ready-set path, got %d calls", delegateCalls)
	}
	if _, err := os.Stat(filepath.Join(home, ".keystone", "toolchain", "state", "current.json")); !os.IsNotExist(err) {
		t.Fatalf("expected no current.json write during dry-run, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".bashrc")); !os.IsNotExist(err) {
		t.Fatalf("expected dry-run not to write shell rc, got err=%v", err)
	}
}

func TestInitReportInheritsDelegatedNonSuccess(t *testing.T) {
	home := t.TempDir()
	testGitRepo(t, filepath.Join(home, "git", "keystone-hub"))
	t.Setenv("SHELL", "/bin/bash")

	svc := New(testServiceContext(home))
	svc.readySetExecutor = func(toolchain.SyncOptions) (toolchain.SyncReport, []contract.Warning, int, *contract.AppError) {
		report := toolchain.SyncReport{
			Schema:  toolchain.SyncReportSchema,
			Outcome: toolchain.SyncOutcomeCompletedWithBlockers,
			Summary: toolchain.SyncSummary{
				ReadyRepoCount:   1,
				BlockedRepoCount: 1,
			},
			PrimaryNextAction: "fix the build or probe failure for keystone-hub, then run `kstoolchain sync`",
			FinalStatus: toolchain.StatusReport{
				Schema:        toolchain.StatusReportSchema,
				ManagedBinDir: svc.ctx.Config.ManagedBinDir,
				Summary: toolchain.StatusSummary{
					Overall:     toolchain.StateFailed,
					StateCounts: map[string]int{toolchain.StateFailed: 1},
				},
				Repos: []toolchain.RepoStatus{
					{
						RepoID:        "keystone-hub",
						AdapterStatus: toolchain.AdapterStatusReady,
						State:         toolchain.StateFailed,
					},
				},
			},
		}
		return report, nil, report.ExitCode(), nil
	}

	var stdout bytes.Buffer
	report, _, exitCode, appErr := svc.InitReport(strings.NewReader("\ny\n"), &stdout, toolchain.InitOptions{})
	if appErr != nil {
		t.Fatalf("init report: %v", appErr)
	}
	if exitCode != contract.ExitReadySetBlocked {
		t.Fatalf("expected delegated non-success to fail init, got %d", exitCode)
	}
	if !report.Delegated || report.ReadySet == nil {
		t.Fatalf("expected delegated ready-set report, got %#v", report)
	}
	lines := toolchain.RenderInitText(report)
	if got := lines[0]; got != "Init: delegated ready-set result completed_with_blockers" {
		t.Fatalf("unexpected init headline: %q", got)
	}
	if !containsString(report.ManualActions, "run `kstoolchain sync`") {
		t.Fatalf("expected delegated next action to name sync, got %#v", report.ManualActions)
	}
}

func TestSyncReportPassesOptionsToSharedReadySetExecutor(t *testing.T) {
	home := t.TempDir()
	svc := New(testServiceContext(home))

	called := false
	svc.readySetExecutor = func(opts toolchain.SyncOptions) (toolchain.SyncReport, []contract.Warning, int, *contract.AppError) {
		called = true
		if !opts.AllowDirty {
			t.Fatalf("expected allow-dirty option to reach shared executor, got %#v", opts)
		}
		report := toolchain.SyncReport{
			Schema:  toolchain.SyncReportSchema,
			Outcome: toolchain.SyncOutcomeNoChange,
			Summary: toolchain.SyncSummary{
				ReadyRepoCount: 1,
			},
			FinalStatus: toolchain.StatusReport{
				Schema:        toolchain.StatusReportSchema,
				ManagedBinDir: svc.ctx.Config.ManagedBinDir,
				Summary: toolchain.StatusSummary{
					Overall:     toolchain.StateCurrent,
					StateCounts: map[string]int{toolchain.StateCurrent: 1},
				},
			},
		}
		return report, nil, report.ExitCode(), nil
	}

	report, warnings, exitCode, appErr := svc.SyncReport(toolchain.SyncOptions{AllowDirty: true})
	if appErr != nil {
		t.Fatalf("sync report: %v", appErr)
	}
	if !called {
		t.Fatal("expected shared ready-set executor call")
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
	if exitCode != contract.ExitOK {
		t.Fatalf("unexpected exit code: got=%d want=%d", exitCode, contract.ExitOK)
	}
	if report.FinalStatus.Summary.Overall != toolchain.StateCurrent {
		t.Fatalf("unexpected overall state: %s", report.FinalStatus.Summary.Overall)
	}
}

func containsString(items []string, needle string) bool {
	for _, item := range items {
		if strings.Contains(item, needle) {
			return true
		}
	}
	return false
}

func testServiceContext(home string) *runtime.Context {
	return &runtime.Context{
		HomeDir: home,
		Config: runtime.Config{
			ManagedBinDir: filepath.Join(home, ".keystone", "toolchain", "active", "bin"),
			StateDir:      filepath.Join(home, ".keystone", "toolchain", "state"),
		},
		Now: func() time.Time {
			return time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
		},
	}
}

func testGitRepo(t *testing.T, repoPath string) {
	t.Helper()
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
}

func runCmd(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, string(out))
	}
}
