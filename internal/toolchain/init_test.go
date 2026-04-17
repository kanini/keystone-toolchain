package toolchain

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kanini/keystone-toolchain/internal/runtime"
)

func TestRunInitFlowDryRunDiscoversHubWithoutWriting(t *testing.T) {
	home := t.TempDir()
	hubRepo := filepath.Join(home, "git", "keystone-hub")
	testGitRepoAt(t, hubRepo)
	t.Setenv("SHELL", "/bin/bash")

	var out bytes.Buffer
	report, appErr := RunInitFlow(testInitContext(home), strings.NewReader(""), &out, InitOptions{DryRun: true})
	if appErr != nil {
		t.Fatalf("run init dry-run: %v", appErr)
	}
	if report.Applied {
		t.Fatal("dry-run must not write the overlay")
	}
	if !report.Changed {
		t.Fatal("expected dry-run to report a semantic diff")
	}
	if _, err := os.Stat(filepath.Join(home, ".keystone", "toolchain", "adapters.yaml")); !os.IsNotExist(err) {
		t.Fatalf("expected no overlay write during dry-run, got err=%v", err)
	}
	foundHub := false
	for _, repo := range report.Repos {
		if repo.RepoID == "keystone-hub" && repo.RepoPath == hubRepo && repo.Source == "discovered" {
			foundHub = true
		}
	}
	if !foundHub {
		t.Fatalf("expected discovered hub repo in report, got %#v", report.Repos)
	}
}

func TestRunInitFlowWritesOverlayAfterRemovingUnknownRows(t *testing.T) {
	home := t.TempDir()
	hubRepo := filepath.Join(home, "git", "keystone-hub")
	testGitRepoAt(t, hubRepo)
	t.Setenv("SHELL", "/bin/bash")

	overlayPath := filepath.Join(home, ".keystone", "toolchain", "adapters.yaml")
	writeOverlayFixture(t, overlayPath, `schema: kstoolchain.adapter-overlay/v1alpha1
repos:
  - repo_id: stale-repo
    repo_path: ~/git/stale-repo
`)

	var out bytes.Buffer
	input := strings.NewReader("\nr\ny\n")
	report, appErr := RunInitFlow(testInitContext(home), input, &out, InitOptions{})
	if appErr != nil {
		t.Fatalf("run init: %v", appErr)
	}
	if !report.Applied {
		t.Fatal("expected overlay write")
	}

	data, err := os.ReadFile(overlayPath)
	if err != nil {
		t.Fatalf("read overlay: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "repo_id: keystone-hub") {
		t.Fatalf("expected hub overlay row, got %s", body)
	}
	if strings.Contains(body, "stale-repo") {
		t.Fatalf("expected stale overlay row to be removed, got %s", body)
	}
}

func testInitContext(home string) *runtime.Context {
	return &runtime.Context{
		HomeDir: home,
		Config: runtime.Config{
			ManagedBinDir: filepath.Join(home, ".keystone", "toolchain", "active", "bin"),
			StateDir:      filepath.Join(home, ".keystone", "toolchain", "state"),
		},
	}
}

func testGitRepoAt(t *testing.T, repoPath string) {
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
