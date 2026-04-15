package toolchain

import (
	"strings"
	"testing"

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
	if manifest.ManagedBinDir == "" {
		t.Fatal("expected managed bin dir")
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
