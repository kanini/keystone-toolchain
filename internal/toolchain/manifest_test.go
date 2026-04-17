package toolchain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kanini/keystone-toolchain/internal/contract"
	"github.com/kanini/keystone-toolchain/internal/runtime"
)

func TestLoadManifest(t *testing.T) {
	ctx := &runtime.Context{HomeDir: t.TempDir()}
	manifest, err := LoadManifest(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if manifest.Schema == "" {
		t.Fatal("expected schema")
	}
	if len(manifest.Repos) < 4 {
		t.Fatalf("expected repo entries, got %d", len(manifest.Repos))
	}
	ready := SelectReadyAdapters(manifest)
	if len(ready) != 1 || ready[0].RepoID != "keystone-hub" {
		t.Fatalf("expected keystone-hub to be the only ready adapter, got %#v", ready)
	}
	if ready[0].RepoPath != "" {
		t.Fatalf("expected missing default overlay to leave repo_path unset, got %q", ready[0].RepoPath)
	}
	if len(manifest.Overlay.Diagnostics) != 1 || manifest.Overlay.Diagnostics[0].Code != contract.CodeOverlayMissing {
		t.Fatalf("expected OVERLAY_MISSING diagnostic, got %#v", manifest.Overlay.Diagnostics)
	}
}

func TestLoadManifestMergesDefaultOverlay(t *testing.T) {
	home := t.TempDir()
	ctx := &runtime.Context{HomeDir: home}

	writeOverlayFixture(t, filepath.Join(home, ".keystone", "toolchain", "adapters.yaml"), `schema: kstoolchain.adapter-overlay/v1alpha1
repos:
  - repo_id: keystone-hub
    repo_path: ~/git/keystone-hub
  - repo_id: keystone-memory
    repo_path: ~/git/keystone-memory
`)

	manifest, appErr := LoadManifest(ctx)
	if appErr != nil {
		t.Fatalf("load manifest: %v", appErr)
	}

	paths := map[string]string{}
	for _, repo := range manifest.Repos {
		paths[repo.RepoID] = repo.RepoPath
	}
	if got := paths["keystone-hub"]; got != filepath.Join(home, "git", "keystone-hub") {
		t.Fatalf("unexpected hub repo_path: %q", got)
	}
	if got := paths["keystone-memory"]; got != filepath.Join(home, "git", "keystone-memory") {
		t.Fatalf("unexpected memory repo_path: %q", got)
	}
}

func TestLoadManifestUsesExplicitOverlaySelection(t *testing.T) {
	home := t.TempDir()
	explicitPath := filepath.Join(home, "custom", "adapters.yaml")
	writeOverlayFixture(t, explicitPath, `schema: kstoolchain.adapter-overlay/v1alpha1
repos:
  - repo_id: keystone-hub
    repo_path: ~/work/hub
`)

	ctx := &runtime.Context{HomeDir: home, AdaptersPath: explicitPath}
	manifest, appErr := LoadManifest(ctx)
	if appErr != nil {
		t.Fatalf("load manifest: %v", appErr)
	}
	var hub RepoAdapter
	for _, repo := range manifest.Repos {
		if repo.RepoID == "keystone-hub" {
			hub = repo
			break
		}
	}
	if hub.RepoPath != filepath.Join(home, "work", "hub") {
		t.Fatalf("unexpected explicit repo_path: %q", hub.RepoPath)
	}
}

func TestLoadManifestUnknownOverlayRowProducesDiagnostic(t *testing.T) {
	home := t.TempDir()
	writeOverlayFixture(t, filepath.Join(home, ".keystone", "toolchain", "adapters.yaml"), `schema: kstoolchain.adapter-overlay/v1alpha1
repos:
  - repo_id: keystone-hub
    repo_path: ~/git/keystone-hub
  - repo_id: stale-repo
    repo_path: ~/git/stale-repo
`)

	manifest, appErr := LoadManifest(&runtime.Context{HomeDir: home})
	if appErr != nil {
		t.Fatalf("load manifest: %v", appErr)
	}
	if len(manifest.Overlay.UnknownRows) != 1 || manifest.Overlay.UnknownRows[0].RepoID != "stale-repo" {
		t.Fatalf("unexpected unknown rows: %#v", manifest.Overlay.UnknownRows)
	}
	warnings := StatusOverlayWarnings(manifest)
	if len(warnings) != 1 || warnings[0].Code != contract.CodeOverlayUnknown {
		t.Fatalf("unexpected warnings: %#v", warnings)
	}
}

func TestLoadManifestRejectsDuplicateOverlayRepoIDs(t *testing.T) {
	home := t.TempDir()
	writeOverlayFixture(t, filepath.Join(home, ".keystone", "toolchain", "adapters.yaml"), `schema: kstoolchain.adapter-overlay/v1alpha1
repos:
  - repo_id: keystone-hub
    repo_path: ~/git/keystone-hub
  - repo_id: keystone-hub
    repo_path: ~/git/keystone-hub-two
`)

	_, appErr := LoadManifest(&runtime.Context{HomeDir: home})
	if appErr == nil {
		t.Fatal("expected duplicate overlay repo_id error")
	}
	if appErr.Code != contract.CodeOverlayDupID {
		t.Fatalf("unexpected error code: %s", appErr.Code)
	}
}

func TestHubAdapterStagesIntoRequestedBin(t *testing.T) {
	ctx := &runtime.Context{HomeDir: t.TempDir()}
	manifest, err := LoadManifest(ctx)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	var hub *RepoAdapter
	for i := range manifest.Repos {
		if manifest.Repos[i].RepoID == "keystone-hub" {
			hub = &manifest.Repos[i]
			break
		}
	}
	if hub == nil {
		t.Fatal("keystone-hub adapter not found in manifest")
	}

	installStr := strings.Join(hub.InstallCmd, " ")

	// Must use build-local (direct output), not `go install` which ignores GOBIN.
	if !strings.Contains(installStr, "build-local") {
		t.Errorf("hub install_cmd must use build-local, got: %v", hub.InstallCmd)
	}
	// Must direct output into the stage dir via {{stage_bin}}.
	if !strings.Contains(installStr, "{{stage_bin}}") {
		t.Errorf("hub install_cmd must reference {{stage_bin}}, got: %v", hub.InstallCmd)
	}

	// Template expansion must embed the requested stage dir into the command.
	stageDir := t.TempDir()
	expanded, appErr := expandCommandArgs(hub.InstallCmd, templateVars{stageBin: stageDir})
	if appErr != nil {
		t.Fatalf("expand install_cmd: %v", appErr)
	}
	if !strings.Contains(strings.Join(expanded, " "), stageDir) {
		t.Errorf("expanded install_cmd must reference stage dir %s, got: %v", stageDir, expanded)
	}
}

func TestSelectReadyAdapters(t *testing.T) {
	manifest := Manifest{
		Repos: []RepoAdapter{
			{RepoID: "a", Status: AdapterStatusCandidate},
			{RepoID: "b", Status: AdapterStatusReady},
			{RepoID: "c", Status: AdapterStatusBlocked},
		},
	}
	ready := SelectReadyAdapters(manifest)
	if len(ready) != 1 {
		t.Fatalf("expected one ready adapter, got %d", len(ready))
	}
	if ready[0].RepoID != "b" {
		t.Fatalf("unexpected ready adapter: %#v", ready[0])
	}
}

func TestValidateManifestRejectsUnsafeArtifactPaths(t *testing.T) {
	manifest := Manifest{
		Schema:        "kstoolchain.adapter/v1alpha1",
		ManagedBinDir: "/tmp/managed",
		Repos: []RepoAdapter{
			{
				RepoID:           "keystone-context",
				RepoPath:         "/tmp/repo",
				InstallCmd:       []string{"make", "build-local", "BIN_DIR={{stage_bin}}"},
				ExpectedOutputs:  []string{"ksctx"},
				SupportArtifacts: []string{"../.ksctx-runtime"},
				ProbeCmd:         []string{"{{stage_bin}}/ksctx", "--version"},
				DirtyPolicy:      DirtyPolicyFailClosed,
				ReleaseUnit:      ReleaseUnitRepo,
				Status:           AdapterStatusReady,
			},
		},
	}

	appErr := validateManifest(manifest)
	if appErr == nil {
		t.Fatal("expected validation error for unsafe artifact path")
	}
	if !strings.Contains(appErr.Message, "must not escape the stage root") {
		t.Fatalf("unexpected validation message: %q", appErr.Message)
	}
}

func writeOverlayFixture(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir overlay dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write overlay: %v", err)
	}
}
