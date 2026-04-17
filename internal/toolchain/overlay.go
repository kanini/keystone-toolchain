package toolchain

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/kanini/keystone-toolchain/internal/contract"
	"github.com/kanini/keystone-toolchain/internal/runtime"
)

const AdapterOverlaySchema = "kstoolchain.adapter-overlay/v1alpha1"

type AdapterOverlay struct {
	Schema string        `yaml:"schema"`
	Repos  []OverlayRepo `yaml:"repos"`
}

type OverlayRepo struct {
	RepoID   string `yaml:"repo_id" json:"repo_id"`
	RepoPath string `yaml:"repo_path" json:"repo_path"`
}

type OverlayDiagnostic struct {
	Code string
	Path string
	Repo OverlayRepo
}

type OverlaySelection struct {
	Path        string
	Explicit    bool
	Present     bool
	Entries     map[string]string
	UnknownRows []OverlayRepo
	Diagnostics []OverlayDiagnostic
}

type overlayLoadOptions struct {
	allowMissing       bool
	emitMissingWarning bool
}

func defaultOverlayPath(home string) string {
	return filepath.Join(home, ".keystone", "toolchain", "adapters.yaml")
}

func resolveOverlayPath(ctx *runtime.Context) (string, bool, *contract.AppError) {
	if strings.TrimSpace(ctx.AdaptersPath) == "" {
		return defaultOverlayPath(ctx.HomeDir), false, nil
	}
	path, err := runtime.NormalizePath(ctx.AdaptersPath, ctx.HomeDir)
	if err != nil || strings.TrimSpace(path) == "" {
		return "", true, contract.Validation(contract.CodeOverlayInvalid, "Could not resolve adapters overlay path.", "Set --adapters to a valid path.", contract.Detail{Name: "path", Value: ctx.AdaptersPath})
	}
	return path, true, nil
}

func loadOverlaySelection(ctx *runtime.Context, knownRepoIDs map[string]struct{}, opts overlayLoadOptions) (OverlaySelection, *contract.AppError) {
	path, explicit, appErr := resolveOverlayPath(ctx)
	if appErr != nil {
		return OverlaySelection{}, appErr
	}

	selection := OverlaySelection{
		Path:     path,
		Explicit: explicit,
		Entries:  map[string]string{},
	}

	rawBytes, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if explicit && !opts.allowMissing {
				return OverlaySelection{}, contract.Validation(contract.CodeOverlayMissing, "Adapters overlay file does not exist.", "Create it with `kstoolchain init` or point --adapters at an existing overlay.", contract.Detail{Name: "path", Value: path})
			}
			if !explicit && opts.emitMissingWarning {
				selection.Diagnostics = append(selection.Diagnostics, OverlayDiagnostic{
					Code: contract.CodeOverlayMissing,
					Path: path,
				})
			}
			return selection, nil
		}
		return OverlaySelection{}, contract.Validation(contract.CodeOverlayIO, "Adapters overlay file could not be read.", "Check file permissions or choose a different overlay file.", contract.Detail{Name: "path", Value: path})
	}

	selection.Present = true
	var overlay AdapterOverlay
	if err := yaml.Unmarshal(rawBytes, &overlay); err != nil {
		return OverlaySelection{}, contract.Validation(contract.CodeOverlayInvalid, "Adapters overlay file is not valid YAML.", "Fix the overlay file and retry.", contract.Detail{Name: "path", Value: path})
	}
	if strings.TrimSpace(overlay.Schema) != AdapterOverlaySchema {
		return OverlaySelection{}, contract.Validation(contract.CodeOverlayInvalid, "Adapters overlay schema must be kstoolchain.adapter-overlay/v1alpha1.", "Fix the overlay file and retry.", contract.Detail{Name: "path", Value: path})
	}

	seen := map[string]struct{}{}
	for _, row := range overlay.Repos {
		repoID := strings.TrimSpace(row.RepoID)
		if repoID == "" {
			return OverlaySelection{}, contract.Validation(contract.CodeOverlayInvalid, "Adapters overlay repo_id is required.", "Fix the overlay file and retry.", contract.Detail{Name: "path", Value: path})
		}
		repoPath, err := runtime.NormalizePath(row.RepoPath, ctx.HomeDir)
		if err != nil || strings.TrimSpace(repoPath) == "" {
			return OverlaySelection{}, contract.Validation(contract.CodeOverlayInvalid, fmt.Sprintf("Adapters overlay row %s must declare repo_path.", repoID), "Fix the overlay file and retry.", contract.Detail{Name: "path", Value: path}, contract.Detail{Name: "repo_id", Value: repoID})
		}
		if _, ok := seen[repoID]; ok {
			return OverlaySelection{}, contract.Validation(contract.CodeOverlayDupID, fmt.Sprintf("Adapters overlay duplicates repo_id %s.", repoID), "Remove the duplicate overlay row and retry.", contract.Detail{Name: "path", Value: path}, contract.Detail{Name: "repo_id", Value: repoID})
		}
		seen[repoID] = struct{}{}

		normalized := OverlayRepo{RepoID: repoID, RepoPath: repoPath}
		if _, ok := knownRepoIDs[repoID]; !ok {
			selection.UnknownRows = append(selection.UnknownRows, normalized)
			selection.Diagnostics = append(selection.Diagnostics, OverlayDiagnostic{
				Code: contract.CodeOverlayUnknown,
				Path: path,
				Repo: normalized,
			})
			continue
		}
		selection.Entries[repoID] = repoPath
	}

	return selection, nil
}

func encodeOverlayDocument(rows []OverlayRepo) ([]byte, *contract.AppError) {
	doc := AdapterOverlay{
		Schema: AdapterOverlaySchema,
		Repos:  append([]OverlayRepo{}, rows...),
	}
	data, err := yaml.Marshal(doc)
	if err != nil {
		return nil, contract.Infra(contract.CodeIOError, "Could not encode adapters overlay file.", "Retry after checking the overlay data.", err)
	}
	return data, nil
}
