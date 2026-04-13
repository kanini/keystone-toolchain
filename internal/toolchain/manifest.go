package toolchain

import (
	_ "embed"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/kanini/keystone-toolchain/internal/contract"
	"github.com/kanini/keystone-toolchain/internal/runtime"
)

//go:embed defaults/adapters.yaml
var defaultManifestBytes []byte

const (
	AdapterStatusReady     = "ready"
	AdapterStatusCandidate = "candidate"
	AdapterStatusBlocked   = "blocked"

	DirtyPolicyFailClosed = "fail_closed"

	ReleaseUnitRepo = "repo"
)

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
	managedBinDir, err := runtime.NormalizePath(manifest.ManagedBinDir, ctx.HomeDir)
	if err != nil {
		return Manifest{}, contract.Validation(contract.CodeConfigInvalid, "Default adapter manifest managed_bin_dir is invalid.", "Fix the embedded adapter manifest.")
	}
	manifest.ManagedBinDir = managedBinDir
	for i := range manifest.Repos {
		repoPath, err := runtime.NormalizePath(manifest.Repos[i].RepoPath, ctx.HomeDir)
		if err != nil {
			return Manifest{}, contract.Validation(contract.CodeConfigInvalid, fmt.Sprintf("Adapter %s repo_path is invalid.", manifest.Repos[i].RepoID), "Fix the embedded adapter manifest.")
		}
		manifest.Repos[i].RepoPath = repoPath
		manifest.Repos[i].Status = normalizeAdapterStatus(manifest.Repos[i].Status)
		manifest.Repos[i].DirtyPolicy = normalizeDirtyPolicy(manifest.Repos[i].DirtyPolicy)
		manifest.Repos[i].ReleaseUnit = normalizeReleaseUnit(manifest.Repos[i].ReleaseUnit)
	}
	if appErr := validateManifest(manifest); appErr != nil {
		return Manifest{}, appErr
	}
	return manifest, nil
}

func validateManifest(manifest Manifest) *contract.AppError {
	if strings.TrimSpace(manifest.Schema) != "kstoolchain.adapter/v1alpha1" {
		return contract.Validation(contract.CodeConfigInvalid, "Adapter manifest schema must be kstoolchain.adapter/v1alpha1.", "Fix the embedded adapter manifest.")
	}
	if strings.TrimSpace(manifest.ManagedBinDir) == "" {
		return contract.Validation(contract.CodeConfigInvalid, "Adapter manifest managed_bin_dir is required.", "Fix the embedded adapter manifest.")
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
		case repo.Status == "":
			return contract.Validation(contract.CodeConfigInvalid, fmt.Sprintf("Adapter %s must declare a valid status.", repo.RepoID), "Fix the embedded adapter manifest.")
		case repo.DirtyPolicy == "":
			return contract.Validation(contract.CodeConfigInvalid, fmt.Sprintf("Adapter %s must declare a valid dirty_policy.", repo.RepoID), "Fix the embedded adapter manifest.")
		case repo.ReleaseUnit == "":
			return contract.Validation(contract.CodeConfigInvalid, fmt.Sprintf("Adapter %s must declare a valid release_unit.", repo.RepoID), "Fix the embedded adapter manifest.")
		case len(repo.ExpectedOutputs) == 0:
			return contract.Validation(contract.CodeConfigInvalid, fmt.Sprintf("Adapter %s must declare expected_outputs.", repo.RepoID), "Fix the embedded adapter manifest.")
		case len(repo.InstallCmd) == 0 && repo.Status == AdapterStatusReady:
			return contract.Validation(contract.CodeConfigInvalid, fmt.Sprintf("Ready adapter %s must declare install_cmd.", repo.RepoID), "Fix the embedded adapter manifest.")
		case len(repo.ProbeCmd) == 0 && repo.Status == AdapterStatusReady:
			return contract.Validation(contract.CodeConfigInvalid, fmt.Sprintf("Ready adapter %s must declare probe_cmd.", repo.RepoID), "Fix the embedded adapter manifest.")
		}
		if _, ok := seen[repo.RepoID]; ok {
			return contract.Validation(contract.CodeConfigInvalid, fmt.Sprintf("Adapter repo_id %s is duplicated.", repo.RepoID), "Fix the embedded adapter manifest.")
		}
		seen[repo.RepoID] = struct{}{}
	}
	return nil
}

func SelectReadyAdapters(manifest Manifest) []RepoAdapter {
	adapters := make([]RepoAdapter, 0, len(manifest.Repos))
	for _, repo := range manifest.Repos {
		if repo.Status == AdapterStatusReady {
			adapters = append(adapters, repo)
		}
	}
	return adapters
}

func normalizeAdapterStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case AdapterStatusReady:
		return AdapterStatusReady
	case AdapterStatusCandidate:
		return AdapterStatusCandidate
	case AdapterStatusBlocked:
		return AdapterStatusBlocked
	default:
		return ""
	}
}

func normalizeDirtyPolicy(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case DirtyPolicyFailClosed:
		return DirtyPolicyFailClosed
	default:
		return ""
	}
}

func normalizeReleaseUnit(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case ReleaseUnitRepo:
		return ReleaseUnitRepo
	default:
		return ""
	}
}
