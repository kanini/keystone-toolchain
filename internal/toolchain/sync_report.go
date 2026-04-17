package toolchain

import (
	"fmt"
	"strings"

	"github.com/kanini/keystone-toolchain/internal/contract"
)

const (
	SyncReportSchema = "kstoolchain.sync-report/v1alpha1"

	SyncOutcomeSucceeded             = "succeeded"
	SyncOutcomeNoChange              = "no_change"
	SyncOutcomeCompletedWithBlockers = "completed_with_blockers"
)

type SyncReport struct {
	Schema            string       `json:"schema"`
	Outcome           string       `json:"outcome"`
	Summary           SyncSummary  `json:"summary"`
	PrimaryNextAction string       `json:"primary_next_action,omitempty"`
	FinalStatus       StatusReport `json:"final_status"`
}

type SyncSummary struct {
	ReadyRepoCount      int `json:"ready_repo_count"`
	BlockedRepoCount    int `json:"blocked_repo_count"`
	UpdatedRepoCount    int `json:"updated_repo_count"`
	ShadowedOutputCount int `json:"shadowed_output_count"`
}

type activeBuildTuple struct {
	present     bool
	activeBuild string
	sourceKind  string
}

func BuildSyncReport(pre PersistedState, post PersistedState, finalStatus StatusReport) SyncReport {
	report := SyncReport{
		Schema:      SyncReportSchema,
		FinalStatus: finalStatus,
		Summary: SyncSummary{
			ReadyRepoCount: finalStatus.Summary.RepoCount,
		},
	}

	preIndex := map[string]PersistedRepoState{}
	for _, repo := range pre.Repos {
		preIndex[repo.RepoID] = repo
	}
	postIndex := map[string]PersistedRepoState{}
	for _, repo := range post.Repos {
		postIndex[repo.RepoID] = repo
	}

	for _, repo := range finalStatus.Repos {
		if repo.State != StateCurrent {
			report.Summary.BlockedRepoCount++
		}
		for _, output := range repo.Outputs {
			if output.State == StateShadowed {
				report.Summary.ShadowedOutputCount++
			}
		}
		if activeBuildTupleForSync(preIndex[repo.RepoID]) != activeBuildTupleForSync(postIndex[repo.RepoID]) {
			report.Summary.UpdatedRepoCount++
		}
	}

	report.Outcome = deriveSyncOutcome(report)
	report.PrimaryNextAction = deriveSyncPrimaryNextAction(report)
	return report
}

func (report SyncReport) EnvelopeOK() bool {
	return report.Outcome != SyncOutcomeCompletedWithBlockers
}

func (report SyncReport) ExitCode() int {
	if report.EnvelopeOK() {
		return contract.ExitOK
	}
	return contract.ExitReadySetBlocked
}

func RenderSyncText(report SyncReport, verbose bool) []string {
	lines := []string{
		fmt.Sprintf("Sync: %s", report.Outcome),
		fmt.Sprintf(
			"Ready repos: %d | Updated: %d | Blocked: %d | Shadowed outputs: %d",
			report.Summary.ReadyRepoCount,
			report.Summary.UpdatedRepoCount,
			report.Summary.BlockedRepoCount,
			report.Summary.ShadowedOutputCount,
		),
		fmt.Sprintf("Final status: %s", report.FinalStatus.Summary.Overall),
	}

	if report.Summary.BlockedRepoCount > 0 {
		lines = append(lines, "Blockers:")
		for _, repo := range report.FinalStatus.Repos {
			if repo.State == StateCurrent {
				continue
			}
			line := fmt.Sprintf("  - %s  state=%s", repo.RepoID, repo.State)
			if reason := strings.TrimSpace(repo.Reason); reason != "" {
				line = fmt.Sprintf("%s  reason=%s", line, reason)
			}
			lines = append(lines, line)
		}
	}
	if report.PrimaryNextAction != "" {
		lines = append(lines, fmt.Sprintf("Next: %s", report.PrimaryNextAction))
	}
	if verbose {
		lines = append(lines, "Final ready-set detail:")
		for _, line := range RenderStatusText(report.FinalStatus) {
			lines = append(lines, "  "+line)
		}
	}

	return lines
}

func CollectSyncManualActions(report SyncReport) []string {
	actions := []string{}
	if report.PrimaryNextAction != "" {
		actions = append(actions, report.PrimaryNextAction)
	}
	if report.Outcome == SyncOutcomeCompletedWithBlockers {
		actions = appendUniqueStrings(actions, CollectReadySetManualActions(report.FinalStatus)...)
	}
	return actions
}

func deriveSyncOutcome(report SyncReport) string {
	if report.FinalStatus.Summary.Overall == StateCurrent && report.Summary.ShadowedOutputCount == 0 {
		if report.Summary.UpdatedRepoCount > 0 {
			return SyncOutcomeSucceeded
		}
		return SyncOutcomeNoChange
	}
	return SyncOutcomeCompletedWithBlockers
}

func deriveSyncPrimaryNextAction(report SyncReport) string {
	switch report.Outcome {
	case SyncOutcomeSucceeded:
		if report.Summary.UpdatedRepoCount > 0 {
			return fmt.Sprintf("open a new shell or source your rc file so %s wins on PATH", report.FinalStatus.ManagedBinDir)
		}
	case SyncOutcomeNoChange:
		return ""
	case SyncOutcomeCompletedWithBlockers:
		actions := CollectReadySetManualActions(report.FinalStatus)
		if len(actions) > 0 {
			return actions[0]
		}
	}
	return ""
}

func activeBuildTupleForSync(repo PersistedRepoState) activeBuildTuple {
	build := strings.TrimSpace(repo.ActiveBuild)
	sourceKind := normalizeSourceKind(repo.ActiveSourceKind)
	if build == "" || sourceKind == "" {
		return activeBuildTuple{}
	}
	return activeBuildTuple{
		present:     true,
		activeBuild: build,
		sourceKind:  sourceKind,
	}
}
