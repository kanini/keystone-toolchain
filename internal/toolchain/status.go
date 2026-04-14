package toolchain

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kanini/keystone-toolchain/internal/contract"
	"github.com/kanini/keystone-toolchain/internal/runtime"
)

const (
	PersistedStateSchema = "kstoolchain.state/v1alpha1"
	StatusReportSchema   = "kstoolchain.status/v1alpha1"

	StateCurrent       = "CURRENT"
	StateStaleLKG      = "STALE_LKG"
	StateFailed        = "FAILED"
	StateDirtySkipped  = "DIRTY_SKIPPED"
	StateShadowed      = "SHADOWED"
	StateContractDrift = "CONTRACT_DRIFT"
	StateUnknown       = "UNKNOWN"
)

type PersistedState struct {
	Schema        string               `json:"schema"`
	ManagedBinDir string               `json:"managed_bin_dir"`
	LastAttemptAt string               `json:"last_attempt_at,omitempty"`
	LastSuccessAt string               `json:"last_success_at,omitempty"`
	Repos         []PersistedRepoState `json:"repos"`
}

type PersistedRepoState struct {
	RepoID      string   `json:"repo_id"`
	State       string   `json:"state"`
	Reason      string   `json:"reason,omitempty"`
	RepoHead    string   `json:"repo_head,omitempty"`
	ActiveBuild string   `json:"active_build,omitempty"`
	Outputs     []string `json:"outputs,omitempty"`
}

type StatusReport struct {
	Schema           string        `json:"schema"`
	Summary          StatusSummary `json:"summary"`
	ManagedBinDir    string        `json:"managed_bin_dir"`
	StateFile        string        `json:"state_file"`
	StateFilePresent bool          `json:"state_file_present"`
	LastAttemptAt    string        `json:"last_attempt_at,omitempty"`
	LastSuccessAt    string        `json:"last_success_at,omitempty"`
	Repos            []RepoStatus  `json:"repos"`
}

type StatusSummary struct {
	Overall          string         `json:"overall"`
	RepoCount        int            `json:"repo_count"`
	OutputCount      int            `json:"output_count"`
	StateCounts      map[string]int `json:"state_counts"`
	BlockedRepoCount int            `json:"blocked_repo_count"`
}

type RepoStatus struct {
	RepoID        string         `json:"repo_id"`
	RepoPath      string         `json:"repo_path"`
	AdapterStatus string         `json:"adapter_status"`
	DirtyPolicy   string         `json:"dirty_policy"`
	ReleaseUnit   string         `json:"release_unit"`
	State         string         `json:"state"`
	Reason        string         `json:"reason,omitempty"`
	RepoHead      string         `json:"repo_head,omitempty"`
	ActiveBuild   string         `json:"active_build,omitempty"`
	Notes         []string       `json:"notes,omitempty"`
	Outputs       []OutputStatus `json:"outputs"`
}

type OutputStatus struct {
	Name         string `json:"name"`
	ExpectedPath string `json:"expected_path"`
	ResolvedPath string `json:"resolved_path,omitempty"`
	State        string `json:"state"`
	Reason       string `json:"reason,omitempty"`
}

func BuildStatusReport(ctx *runtime.Context, manifest Manifest, persisted PersistedState, stateFile string, stateFilePresent bool, contractDrift bool) StatusReport {
	report := StatusReport{
		Schema:           StatusReportSchema,
		ManagedBinDir:    ctx.Config.ManagedBinDir,
		StateFile:        stateFile,
		StateFilePresent: stateFilePresent,
		LastAttemptAt:    persisted.LastAttemptAt,
		LastSuccessAt:    persisted.LastSuccessAt,
		Summary: StatusSummary{
			Overall:     StateCurrent,
			StateCounts: map[string]int{},
		},
	}

	index := map[string]PersistedRepoState{}
	for _, repo := range persisted.Repos {
		index[repo.RepoID] = repo
	}

	for _, adapter := range manifest.Repos {
		repoState := RepoStatus{
			RepoID:        adapter.RepoID,
			RepoPath:      adapter.RepoPath,
			AdapterStatus: adapter.Status,
			DirtyPolicy:   adapter.DirtyPolicy,
			ReleaseUnit:   adapter.ReleaseUnit,
			Notes:         append([]string{}, adapter.Notes...),
		}

		// Non-ready adapters are not in the managed sync scope. Skip the
		// persisted-state lookup entirely — any stale state from a previous
		// ready-set sync (e.g., if the adapter was demoted from ready to candidate)
		// would be misleading here.
		if adapter.Status != AdapterStatusReady {
			repoState.State = StateUnknown
			if adapter.Status == AdapterStatusBlocked {
				repoState.Reason = "adapter is blocked pending prerequisite work"
			} else {
				repoState.Reason = "adapter is not in the managed sync scope"
			}
		} else {
			persistedRepo, hasPersisted := index[adapter.RepoID]
			switch {
			case contractDrift:
				repoState.State = StateContractDrift
				repoState.Reason = "persisted state managed_bin_dir does not match active configuration"
			case !stateFilePresent:
				repoState.State = StateUnknown
				repoState.Reason = "no persisted toolchain state yet"
			case !hasPersisted:
				repoState.State = StateUnknown
				repoState.Reason = "repo missing from persisted toolchain state"
			default:
				repoState.State = normalizedState(persistedRepo.State)
				repoState.Reason = persistedRepo.Reason
				repoState.RepoHead = persistedRepo.RepoHead
				repoState.ActiveBuild = persistedRepo.ActiveBuild
			}
		}
		if adapter.Status == AdapterStatusReady {
			if liveHead, ok := lookupRepoHead(adapter.RepoPath); ok {
				repoState.RepoHead = liveHead
				if repoState.State == StateCurrent && repoState.ActiveBuild != "" && liveHead != repoState.ActiveBuild {
					repoState.State = StateStaleLKG
					if repoState.Reason == "" {
						repoState.Reason = fmt.Sprintf("repo HEAD %s is ahead of active build %s", shortValue(liveHead), shortValue(repoState.ActiveBuild))
					}
				}
			}
		}

		for _, output := range adapter.ExpectedOutputs {
			item := OutputStatus{
				Name:         output,
				ExpectedPath: filepath.Join(ctx.Config.ManagedBinDir, output),
				State:        repoState.State,
			}
			if item.State == "" {
				item.State = StateUnknown
			}

			// PATH resolution is only meaningful for ready adapters that have an
			// actual managed binary. For candidate and blocked adapters there is no
			// managed-binary referent yet, so SHADOWED would be semantically wrong.
			if adapter.Status == AdapterStatusReady {
				resolved, err := exec.LookPath(output)
				if err == nil {
					item.ResolvedPath = resolved
				}

				switch {
				case item.ResolvedPath == "":
					item.State = demoteMissingPathState(item.State)
					if item.Reason == "" {
						item.Reason = "tool is not on PATH"
					}
				case item.ResolvedPath != item.ExpectedPath:
					item.State = StateShadowed
					item.Reason = fmt.Sprintf("PATH resolves %s instead of %s", item.ResolvedPath, item.ExpectedPath)
				case item.State == StateUnknown && stateFilePresent:
					item.Reason = "managed path resolves correctly but persisted repo state is unknown"
				}
			}
			repoState.Outputs = append(repoState.Outputs, item)
			if adapter.Status == AdapterStatusReady {
				report.Summary.OutputCount++
			}
		}

		repoState.State = deriveRepoState(repoState)
		report.Repos = append(report.Repos, repoState)
		report.Summary.RepoCount++
		if adapter.Status == AdapterStatusReady {
			report.Summary.StateCounts[repoState.State]++
		}
		if adapter.Status == AdapterStatusBlocked {
			report.Summary.BlockedRepoCount++
		}
	}

	report.Summary.Overall = deriveOverall(report.Summary.StateCounts)
	return report
}

// LoadPersistedState reads current.json and returns the persisted state.
// The five return values are: state, stateFile, present, contractDrift, err.
// contractDrift=true means the file exists but its managed_bin_dir does not match
// the active configuration — BuildStatusReport should surface CONTRACT_DRIFT.
func LoadPersistedState(ctx *runtime.Context) (PersistedState, string, bool, bool, *contract.AppError) {
	stateFile := filepath.Join(ctx.Config.StateDir, "current.json")
	data, err := os.ReadFile(stateFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return PersistedState{}, stateFile, false, false, nil
		}
		return PersistedState{}, stateFile, false, false, contract.Infra(contract.CodeIOError, "Could not read toolchain state file.", "Check file permissions and retry.", err, contract.Detail{Name: "path", Value: stateFile})
	}

	var persisted PersistedState
	if err := json.Unmarshal(data, &persisted); err != nil {
		return PersistedState{}, stateFile, true, false, contract.Validation(contract.CodeConfigInvalid, "Toolchain state file is not valid JSON.", "Fix or remove the state file.", contract.Detail{Name: "path", Value: stateFile})
	}
	if appErr := validatePersistedStateShape(persisted); appErr != nil {
		return PersistedState{}, stateFile, true, false, appErr
	}
	matchesConfig, appErr := persistedStateMatchesActiveConfig(ctx, persisted)
	if appErr != nil {
		return PersistedState{}, stateFile, true, false, appErr
	}
	if !matchesConfig {
		// State file exists but was written for a different managed_bin_dir.
		// Signal contract drift so BuildStatusReport can surface CONTRACT_DRIFT.
		return PersistedState{}, stateFile, true, true, nil
	}
	return persisted, stateFile, true, false, nil
}

func SavePersistedState(ctx *runtime.Context, persisted PersistedState) (string, *contract.AppError) {
	if err := os.MkdirAll(ctx.Config.StateDir, 0o755); err != nil {
		return "", contract.Infra(contract.CodeIOError, "Could not create toolchain state dir.", "Check permissions for the state dir and retry.", err, contract.Detail{Name: "path", Value: ctx.Config.StateDir})
	}

	persisted.Schema = PersistedStateSchema
	persisted.ManagedBinDir = ctx.Config.ManagedBinDir
	if appErr := validatePersistedStateShape(persisted); appErr != nil {
		return "", appErr
	}
	if _, appErr := persistedStateMatchesActiveConfig(ctx, persisted); appErr != nil {
		return "", appErr
	}

	tmpFile, err := os.CreateTemp(ctx.Config.StateDir, "current-*.tmp")
	if err != nil {
		return "", contract.Infra(contract.CodeIOError, "Could not create temp state file.", "Check permissions for the state dir and retry.", err, contract.Detail{Name: "path", Value: ctx.Config.StateDir})
	}
	tmpPath := tmpFile.Name()
	enc := json.NewEncoder(tmpFile)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(persisted); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return "", contract.Infra(contract.CodeIOError, "Could not encode toolchain state file.", "Retry after fixing the state payload.", err, contract.Detail{Name: "path", Value: tmpPath})
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", contract.Infra(contract.CodeIOError, "Could not close temp state file.", "Retry after checking filesystem health.", err, contract.Detail{Name: "path", Value: tmpPath})
	}

	stateFile := filepath.Join(ctx.Config.StateDir, "current.json")
	if err := os.Rename(tmpPath, stateFile); err != nil {
		_ = os.Remove(tmpPath)
		return "", contract.Infra(contract.CodeIOError, "Could not atomically write toolchain state file.", "Check that the state dir is writable and retry.", err, contract.Detail{Name: "path", Value: stateFile})
	}

	return stateFile, nil
}

func RenderStatusText(report StatusReport) []string {
	lines := []string{
		fmt.Sprintf("Suite: %s", report.Summary.Overall),
		fmt.Sprintf("Managed bin: %s", report.ManagedBinDir),
		fmt.Sprintf("State file: %s", report.StateFile),
	}
	if report.LastAttemptAt != "" {
		lines = append(lines, fmt.Sprintf("Last attempt: %s", report.LastAttemptAt))
	}
	if report.LastSuccessAt != "" {
		lines = append(lines, fmt.Sprintf("Last success: %s", report.LastSuccessAt))
	}
	lines = append(lines, fmt.Sprintf("Repos: %d | Managed outputs: %d | Blocked adapters: %d", report.Summary.RepoCount, report.Summary.OutputCount, report.Summary.BlockedRepoCount))

	for _, repo := range report.Repos {
		lines = append(lines, fmt.Sprintf("%s  adapter=%s  state=%s", repo.RepoID, repo.AdapterStatus, repo.State))
		if repo.AdapterStatus != AdapterStatusReady {
			lines = append(lines, "  not in sync scope for managed-bin evaluation")
		} else if repo.Reason != "" {
			lines = append(lines, fmt.Sprintf("  reason: %s", repo.Reason))
		}
		lines = append(lines, fmt.Sprintf("  repo: %s", repo.RepoPath))
		if repo.AdapterStatus == AdapterStatusReady {
			for _, output := range repo.Outputs {
				line := fmt.Sprintf("  output %s  %s", output.Name, output.State)
				lines = append(lines, line)
				lines = append(lines, fmt.Sprintf("    expected: %s", output.ExpectedPath))
				if output.ResolvedPath != "" {
					lines = append(lines, fmt.Sprintf("    resolved: %s", output.ResolvedPath))
				}
				if output.Reason != "" {
					lines = append(lines, fmt.Sprintf("    reason: %s", output.Reason))
				}
			}
		}
	}

	return lines
}

func StatusExitCode(report StatusReport) int {
	if report.Summary.Overall == StateCurrent {
		return contract.ExitOK
	}
	return contract.ExitValidation
}

func SyncExitCode(manifest Manifest, persisted PersistedState) int {
	if len(SelectReadyAdapters(manifest)) == 0 {
		return contract.ExitValidation
	}
	index := map[string]PersistedRepoState{}
	for _, repo := range persisted.Repos {
		index[repo.RepoID] = repo
	}
	for _, adapter := range SelectReadyAdapters(manifest) {
		repo, ok := index[adapter.RepoID]
		if !ok {
			return contract.ExitValidation
		}
		if normalizedState(repo.State) != StateCurrent {
			return contract.ExitValidation
		}
	}
	return contract.ExitOK
}

func normalizedState(raw string) string {
	switch strings.TrimSpace(raw) {
	case StateCurrent, StateStaleLKG, StateFailed, StateDirtySkipped, StateShadowed, StateContractDrift, StateUnknown:
		return raw
	default:
		return StateUnknown
	}
}

func deriveRepoState(repo RepoStatus) string {
	state := normalizedState(repo.State)
	for _, output := range repo.Outputs {
		if output.State == StateShadowed {
			return StateShadowed
		}
		if output.State == StateUnknown && (state == StateCurrent || state == StateStaleLKG) {
			return StateUnknown
		}
	}
	return state
}

func deriveOverall(counts map[string]int) string {
	if len(counts) == 0 {
		return StateUnknown
	}
	for _, state := range []string{StateFailed, StateShadowed, StateContractDrift, StateDirtySkipped, StateStaleLKG, StateUnknown} {
		if counts[state] > 0 {
			return state
		}
	}
	return StateCurrent
}

func validatePersistedStateShape(persisted PersistedState) *contract.AppError {
	if strings.TrimSpace(persisted.Schema) != PersistedStateSchema {
		return contract.Validation(contract.CodeConfigInvalid, "Toolchain state schema is invalid.", "Remove or fix the state file.", contract.Detail{Name: "schema", Value: persisted.Schema})
	}
	if strings.TrimSpace(persisted.ManagedBinDir) == "" {
		return contract.Validation(contract.CodeConfigInvalid, "Toolchain state managed_bin_dir is required.", "Remove or fix the state file.")
	}
	seen := map[string]struct{}{}
	for _, repo := range persisted.Repos {
		if strings.TrimSpace(repo.RepoID) == "" {
			return contract.Validation(contract.CodeConfigInvalid, "Toolchain state repo_id is required.", "Remove or fix the state file.")
		}
		if normalizedState(repo.State) != repo.State {
			return contract.Validation(contract.CodeConfigInvalid, fmt.Sprintf("Toolchain state for %s has an invalid state.", repo.RepoID), "Remove or fix the state file.", contract.Detail{Name: "state", Value: repo.State})
		}
		if _, ok := seen[repo.RepoID]; ok {
			return contract.Validation(contract.CodeConfigInvalid, fmt.Sprintf("Toolchain state duplicates repo_id %s.", repo.RepoID), "Remove or fix the state file.")
		}
		seen[repo.RepoID] = struct{}{}
	}
	return nil
}

func persistedStateMatchesActiveConfig(ctx *runtime.Context, persisted PersistedState) (bool, *contract.AppError) {
	managedBinDir, err := runtime.NormalizePath(persisted.ManagedBinDir, ctx.HomeDir)
	if err != nil {
		return false, contract.Validation(contract.CodeConfigInvalid, "Toolchain state managed_bin_dir is invalid.", "Remove or fix the state file.", contract.Detail{Name: "managed_bin_dir", Value: persisted.ManagedBinDir})
	}
	if managedBinDir != ctx.Config.ManagedBinDir {
		return false, nil
	}
	return true, nil
}

func demoteMissingPathState(current string) string {
	switch current {
	case StateCurrent, StateStaleLKG:
		return StateUnknown
	default:
		return current
	}
}

func lookupRepoHead(repoPath string) (string, bool) {
	gitPath, ok := resolveGitBinary()
	if !ok {
		return "", false
	}
	cmd := exec.Command(gitPath, "-C", repoPath, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

func resolveGitBinary() (string, bool) {
	if gitPath, err := exec.LookPath("git"); err == nil {
		return gitPath, true
	}
	for _, candidate := range []string{"/usr/bin/git", "/opt/homebrew/bin/git", "/usr/local/bin/git"} {
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		return candidate, true
	}
	return "", false
}

func shortValue(v string) string {
	v = strings.TrimSpace(v)
	if len(v) > 7 {
		return v[:7]
	}
	if v == "" {
		return "unknown"
	}
	return v
}
