package toolchain

import (
	_ "embed"
	"fmt"
	"path/filepath"
	"sort"
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
	Schema        string           `yaml:"schema" json:"schema"`
	ManagedBinDir string           `yaml:"managed_bin_dir" json:"managed_bin_dir"`
	Repos         []RepoAdapter    `yaml:"repos" json:"repos"`
	Overlay       OverlaySelection `yaml:"-" json:"-"`
}

type RepoAdapter struct {
	RepoID          string   `yaml:"repo_id" json:"repo_id"`
	RepoPath        string   `yaml:"repo_path" json:"repo_path"`
	InstallCmd      []string `yaml:"install_cmd" json:"install_cmd,omitempty"`
	ExpectedOutputs []string `yaml:"expected_outputs" json:"expected_outputs"`
	// SupportArtifacts are non-PATH members of a repo release unit. They are
	// promoted with the executables, but status continues to audit only the
	// PATH-facing expected_outputs surface.
	SupportArtifacts []string          `yaml:"support_artifacts" json:"support_artifacts,omitempty"`
	ProbeCmd         []string          `yaml:"probe_cmd" json:"probe_cmd,omitempty"`
	DirtyPolicy      string            `yaml:"dirty_policy" json:"dirty_policy"`
	ReleaseUnit      string            `yaml:"release_unit" json:"release_unit"`
	Status           string            `yaml:"status" json:"status"`
	Notes            []string          `yaml:"notes" json:"notes,omitempty"`
	Env              map[string]string `yaml:"env" json:"env,omitempty"`
}

func LoadManifest(ctx *runtime.Context) (Manifest, *contract.AppError) {
	manifest, knownRepoIDs, appErr := loadTrackedManifest()
	if appErr != nil {
		return Manifest{}, appErr
	}

	overlay, appErr := loadOverlaySelection(ctx, knownRepoIDs, overlayLoadOptions{
		allowMissing:       false,
		emitMissingWarning: true,
	})
	if appErr != nil {
		return Manifest{}, appErr
	}

	manifest.Overlay = overlay
	for i := range manifest.Repos {
		manifest.Repos[i].RepoPath = overlay.Entries[manifest.Repos[i].RepoID]
	}
	return manifest, nil
}

func loadTrackedManifest() (Manifest, map[string]struct{}, *contract.AppError) {
	var manifest Manifest
	if err := yaml.Unmarshal(defaultManifestBytes, &manifest); err != nil {
		return Manifest{}, nil, contract.Infra(contract.CodeIOError, "Default adapter manifest is invalid.", "Fix the embedded manifest.", err)
	}
	for i := range manifest.Repos {
		manifest.Repos[i].Status = normalizeAdapterStatus(manifest.Repos[i].Status)
		manifest.Repos[i].DirtyPolicy = normalizeDirtyPolicy(manifest.Repos[i].DirtyPolicy)
		manifest.Repos[i].ReleaseUnit = normalizeReleaseUnit(manifest.Repos[i].ReleaseUnit)
		manifest.Repos[i].RepoPath = ""
	}
	if appErr := validateManifest(manifest); appErr != nil {
		return Manifest{}, nil, appErr
	}

	knownRepoIDs := make(map[string]struct{}, len(manifest.Repos))
	for _, repo := range manifest.Repos {
		knownRepoIDs[repo.RepoID] = struct{}{}
	}
	return manifest, knownRepoIDs, nil
}

func validateManifest(manifest Manifest) *contract.AppError {
	if strings.TrimSpace(manifest.Schema) != "kstoolchain.adapter/v1alpha1" {
		return contract.Validation(contract.CodeConfigInvalid, "Adapter manifest schema must be kstoolchain.adapter/v1alpha1.", "Fix the embedded adapter manifest.")
	}
	if len(manifest.Repos) == 0 {
		return contract.Validation(contract.CodeConfigInvalid, "Adapter manifest must declare at least one repo.", "Fix the embedded adapter manifest.")
	}
	seen := map[string]struct{}{}
	for _, repo := range manifest.Repos {
		switch {
		case strings.TrimSpace(repo.RepoID) == "":
			return contract.Validation(contract.CodeConfigInvalid, "Adapter repo_id is required.", "Fix the embedded adapter manifest.")
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
		if appErr := validateArtifactPaths(repo.RepoID, "expected_outputs", repo.ExpectedOutputs); appErr != nil {
			return appErr
		}
		if appErr := validateArtifactPaths(repo.RepoID, "support_artifacts", repo.SupportArtifacts); appErr != nil {
			return appErr
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

func promotedArtifacts(repo RepoAdapter) []string {
	artifacts := make([]string, 0, len(repo.ExpectedOutputs)+len(repo.SupportArtifacts))
	seen := map[string]struct{}{}
	for _, list := range [][]string{repo.ExpectedOutputs, repo.SupportArtifacts} {
		for _, item := range list {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			artifacts = append(artifacts, item)
		}
	}
	return artifacts
}

func validateArtifactPaths(repoID, field string, items []string) *contract.AppError {
	for _, item := range items {
		raw := strings.TrimSpace(item)
		switch {
		case raw == "":
			return contract.Validation(contract.CodeConfigInvalid, fmt.Sprintf("Adapter %s %s entries must not be empty.", repoID, field), "Fix the embedded adapter manifest.")
		case filepath.IsAbs(raw):
			return contract.Validation(contract.CodeConfigInvalid, fmt.Sprintf("Adapter %s %s entry %q must be relative.", repoID, field, raw), "Fix the embedded adapter manifest.")
		}
		clean := filepath.Clean(raw)
		switch {
		case clean != raw:
			return contract.Validation(contract.CodeConfigInvalid, fmt.Sprintf("Adapter %s %s entry %q must be clean and normalized.", repoID, field, raw), "Fix the embedded adapter manifest.")
		case clean == "." || clean == "..":
			return contract.Validation(contract.CodeConfigInvalid, fmt.Sprintf("Adapter %s %s entry %q is invalid.", repoID, field, raw), "Fix the embedded adapter manifest.")
		case strings.HasPrefix(clean, ".."+string(filepath.Separator)):
			return contract.Validation(contract.CodeConfigInvalid, fmt.Sprintf("Adapter %s %s entry %q must not escape the stage root.", repoID, field, raw), "Fix the embedded adapter manifest.")
		}
	}
	return nil
}

func StatusOverlayWarnings(manifest Manifest) []contract.Warning {
	return overlayWarnings(manifest, true)
}

func SyncOverlayWarnings(manifest Manifest) []contract.Warning {
	return overlayWarnings(manifest, false)
}

func SyncOverlayError(manifest Manifest) *contract.AppError {
	if len(manifest.Overlay.UnknownRows) == 0 {
		return nil
	}
	repoIDs := make([]string, 0, len(manifest.Overlay.UnknownRows))
	details := []contract.Detail{{Name: "path", Value: manifest.Overlay.Path}}
	for _, row := range manifest.Overlay.UnknownRows {
		repoIDs = append(repoIDs, row.RepoID)
		details = append(details, contract.Detail{Name: "repo_id", Value: row.RepoID})
	}
	sort.Strings(repoIDs)
	return contract.Validation(
		contract.CodeOverlayUnknown,
		fmt.Sprintf("Adapters overlay contains unknown repo_ids: %s.", strings.Join(repoIDs, ", ")),
		"Remove the stale rows or rerun `kstoolchain init` before syncing.",
		details...,
	)
}

func overlayWarnings(manifest Manifest, includeUnknown bool) []contract.Warning {
	warnings := []contract.Warning{}
	hasMissing := false
	for _, diag := range manifest.Overlay.Diagnostics {
		switch diag.Code {
		case contract.CodeOverlayMissing:
			if hasMissing {
				continue
			}
			hasMissing = true
			warnings = append(warnings, contract.Warning{
				Code:    contract.CodeOverlayMissing,
				Message: fmt.Sprintf("Adapters overlay file is missing: %s", manifest.Overlay.Path),
				Hint:    "Run `kstoolchain init` to create the local repo-path overlay.",
			})
		}
	}

	if includeUnknown && len(manifest.Overlay.UnknownRows) > 0 {
		repoIDs := make([]string, 0, len(manifest.Overlay.UnknownRows))
		for _, row := range manifest.Overlay.UnknownRows {
			repoIDs = append(repoIDs, row.RepoID)
		}
		sort.Strings(repoIDs)
		warnings = append(warnings, contract.Warning{
			Code:    contract.CodeOverlayUnknown,
			Message: fmt.Sprintf("Adapters overlay contains unknown repo_ids: %s.", strings.Join(repoIDs, ", ")),
			Hint:    "Remove the stale rows or rerun `kstoolchain init` to rewrite the overlay.",
		})
	}

	return warnings
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
