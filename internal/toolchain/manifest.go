package toolchain

import (
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/kanini/keystone-toolchain/internal/contract"
	"github.com/kanini/keystone-toolchain/internal/runtime"
)

//go:embed defaults/adapters.yaml
var defaultManifestBytes []byte

type Manifest struct {
	Schema        string        `yaml:"schema" json:"schema"`
	ManagedBinDir string        `yaml:"managed_bin_dir" json:"managed_bin_dir"`
	Repos         []RepoAdapter `yaml:"repos" json:"repos"`
}

type RepoAdapter struct {
	RepoID          string            `yaml:"repo_id" json:"repo_id"`
	RepoPath        string            `yaml:"repo_path" json:"repo_path"`
	InstallCmd      []string          `yaml:"install_cmd" json:"install_cmd,omitempty"`
	ExpectedOutputs []string          `yaml:"expected_outputs" json:"expected_outputs"`
	ProbeCmd        []string          `yaml:"probe_cmd" json:"probe_cmd,omitempty"`
	DirtyPolicy     string            `yaml:"dirty_policy" json:"dirty_policy"`
	ReleaseUnit     string            `yaml:"release_unit" json:"release_unit"`
	Status          string            `yaml:"status" json:"status"`
	Notes           []string          `yaml:"notes" json:"notes,omitempty"`
	Env             map[string]string `yaml:"env" json:"env,omitempty"`
}

func LoadManifest(ctx *runtime.Context) (Manifest, *contract.AppError) {
	var manifest Manifest
	if err := yaml.Unmarshal(defaultManifestBytes, &manifest); err != nil {
		return Manifest{}, contract.Infra(contract.CodeIOError, "Default adapter manifest is invalid.", "Fix the embedded manifest.", err)
	}
	manifest.ManagedBinDir = expandHome(manifest.ManagedBinDir, ctx.HomeDir)
	for i := range manifest.Repos {
		manifest.Repos[i].RepoPath = expandHome(manifest.Repos[i].RepoPath, ctx.HomeDir)
	}
	if appErr := validateManifest(manifest); appErr != nil {
		return Manifest{}, appErr
	}
	return manifest, nil
}

func validateManifest(manifest Manifest) *contract.AppError {
	if manifest.Schema == "" {
		return contract.Validation(contract.CodeConfigInvalid, "Adapter manifest schema is required.", "Fix the embedded adapter manifest.")
	}
	if len(manifest.Repos) == 0 {
		return contract.Validation(contract.CodeConfigInvalid, "Adapter manifest must declare at least one repo.", "Fix the embedded adapter manifest.")
	}
	seen := map[string]struct{}{}
	for _, repo := range manifest.Repos {
		switch {
		case strings.TrimSpace(repo.RepoID) == "":
			return contract.Validation(contract.CodeConfigInvalid, "Adapter repo_id is required.", "Fix the embedded adapter manifest.")
		case strings.TrimSpace(repo.RepoPath) == "":
			return contract.Validation(contract.CodeConfigInvalid, fmt.Sprintf("Adapter %s is missing repo_path.", repo.RepoID), "Fix the embedded adapter manifest.")
		case len(repo.ExpectedOutputs) == 0:
			return contract.Validation(contract.CodeConfigInvalid, fmt.Sprintf("Adapter %s must declare expected_outputs.", repo.RepoID), "Fix the embedded adapter manifest.")
		}
		if _, ok := seen[repo.RepoID]; ok {
			return contract.Validation(contract.CodeConfigInvalid, fmt.Sprintf("Adapter repo_id %s is duplicated.", repo.RepoID), "Fix the embedded adapter manifest.")
		}
		seen[repo.RepoID] = struct{}{}
	}
	return nil
}

func expandHome(rawPath, home string) string {
	switch {
	case rawPath == "~":
		return home
	case strings.HasPrefix(rawPath, "~/"):
		return filepath.Join(home, rawPath[2:])
	default:
		return rawPath
	}
}
