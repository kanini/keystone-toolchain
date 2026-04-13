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

func BuildStatusReport(ctx *runtime.Context, manifest Manifest, persisted PersistedState, stateFile string, stateFilePresent bool) StatusReport {
	report := StatusReport{
		Schema:           "kstoolchain.status/v1alpha1",
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

		persistedRepo, hasPersisted := index[adapter.RepoID]
		switch {
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

		if strings.EqualFold(adapter.Status, "blocked") && repoState.Reason == "" {
			repoState.Reason = "adapter is blocked pending prerequisite work"
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

			resolved, err := exec.LookPath(output)
			if err == nil {
				item.ResolvedPath = resolved
			}

			switch {
			case item.ResolvedPath == "":
				if item.Reason == "" {
					item.Reason = "tool is not on PATH"
				}
			case item.ResolvedPath != item.ExpectedPath:
				item.State = StateShadowed
				item.Reason = fmt.Sprintf("PATH resolves %s instead of %s", item.ResolvedPath, item.ExpectedPath)
			case item.State == StateUnknown && stateFilePresent:
				item.Reason = "managed path resolves correctly but persisted repo state is unknown"
			}
			repoState.Outputs = append(repoState.Outputs, item)
			report.Summary.OutputCount++
		}

		repoState.State = deriveRepoState(repoState)
		report.Repos = append(report.Repos, repoState)
		report.Summary.RepoCount++
		report.Summary.StateCounts[repoState.State]++
		if strings.EqualFold(adapter.Status, "blocked") {
			report.Summary.BlockedRepoCount++
		}
	}

	report.Summary.Overall = deriveOverall(report.Summary.StateCounts)
	return report
}

func LoadPersistedState(ctx *runtime.Context) (PersistedState, string, bool, *contract.AppError) {
	stateFile := filepath.Join(ctx.Config.StateDir, "current.json")
	data, err := os.ReadFile(stateFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return PersistedState{}, stateFile, false, nil
		}
		return PersistedState{}, stateFile, false, contract.Infra(contract.CodeIOError, "Could not read toolchain state file.", "Check file permissions and retry.", err, contract.Detail{Name: "path", Value: stateFile})
	}

	var persisted PersistedState
	if err := json.Unmarshal(data, &persisted); err != nil {
		return PersistedState{}, stateFile, true, contract.Validation(contract.CodeConfigInvalid, "Toolchain state file is not valid JSON.", "Fix or remove the state file.", contract.Detail{Name: "path", Value: stateFile})
	}
	return persisted, stateFile, true, nil
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
	lines = append(lines, fmt.Sprintf("Repos: %d | Outputs: %d | Blocked adapters: %d", report.Summary.RepoCount, report.Summary.OutputCount, report.Summary.BlockedRepoCount))

	for _, repo := range report.Repos {
		lines = append(lines, fmt.Sprintf("%s  %s  adapter=%s", repo.RepoID, repo.State, repo.AdapterStatus))
		if repo.Reason != "" {
			lines = append(lines, fmt.Sprintf("  reason: %s", repo.Reason))
		}
		lines = append(lines, fmt.Sprintf("  repo: %s", repo.RepoPath))
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

	return lines
}

func StatusExitCode(report StatusReport) int {
	if report.Summary.Overall == StateCurrent {
		return contract.ExitOK
	}
	return contract.ExitValidation
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
	if state == "" {
		state = StateUnknown
	}
	for _, output := range repo.Outputs {
		if output.State == StateShadowed {
			return StateShadowed
		}
	}
	return state
}

func deriveOverall(counts map[string]int) string {
	for _, state := range []string{StateShadowed, StateFailed, StateContractDrift, StateDirtySkipped, StateStaleLKG, StateUnknown} {
		if counts[state] > 0 {
			return state
		}
	}
	return StateCurrent
}
