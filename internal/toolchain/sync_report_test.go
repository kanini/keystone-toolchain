package toolchain

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/kanini/keystone-toolchain/internal/contract"
)

func TestBuildSyncReportNoChangeUsesDurableActiveTuple(t *testing.T) {
	finalStatus := StatusReport{
		Schema:        StatusReportSchema,
		ManagedBinDir: "/tmp/bin",
		Summary: StatusSummary{
			Overall:     StateCurrent,
			RepoCount:   1,
			StateCounts: map[string]int{StateCurrent: 1},
		},
		Repos: []RepoStatus{
			{
				RepoID:        "keystone-hub",
				AdapterStatus: AdapterStatusReady,
				State:         StateCurrent,
				Outputs: []OutputStatus{
					{Name: "kshub", ExpectedPath: "/tmp/bin/kshub", State: StateCurrent},
				},
			},
		},
	}
	pre := PersistedState{
		Repos: []PersistedRepoState{
			{
				RepoID:           "keystone-hub",
				ActiveBuild:      "deadbeef",
				ActiveSourceKind: SourceKindCleanHead,
			},
		},
	}
	post := PersistedState{
		Repos: []PersistedRepoState{
			{
				RepoID:           "keystone-hub",
				ActiveBuild:      "deadbeef",
				ActiveSourceKind: SourceKindCleanHead,
			},
		},
	}

	report := BuildSyncReport(pre, post, finalStatus)
	if got := report.Outcome; got != SyncOutcomeNoChange {
		t.Fatalf("unexpected outcome: %s", got)
	}
	if got := report.Summary.UpdatedRepoCount; got != 0 {
		t.Fatalf("unexpected updated repo count: %d", got)
	}
	if report.PrimaryNextAction != "" {
		t.Fatalf("no_change must not emit a primary next action, got %q", report.PrimaryNextAction)
	}
	if got := report.ExitCode(); got != 0 {
		t.Fatalf("unexpected exit code: %d", got)
	}
}

func TestBuildSyncReportSucceededWhenActiveTupleChanges(t *testing.T) {
	finalStatus := StatusReport{
		Schema:        StatusReportSchema,
		ManagedBinDir: "/tmp/bin",
		Summary: StatusSummary{
			Overall:     StateCurrent,
			RepoCount:   1,
			StateCounts: map[string]int{StateCurrent: 1},
		},
		Repos: []RepoStatus{
			{
				RepoID:        "keystone-hub",
				AdapterStatus: AdapterStatusReady,
				State:         StateCurrent,
				Outputs: []OutputStatus{
					{Name: "kshub", ExpectedPath: "/tmp/bin/kshub", State: StateCurrent},
				},
			},
		},
	}
	pre := PersistedState{}
	post := PersistedState{
		Repos: []PersistedRepoState{
			{
				RepoID:           "keystone-hub",
				ActiveBuild:      "deadbeef",
				ActiveSourceKind: SourceKindCleanHead,
			},
		},
	}

	report := BuildSyncReport(pre, post, finalStatus)
	if got := report.Outcome; got != SyncOutcomeSucceeded {
		t.Fatalf("unexpected outcome: %s", got)
	}
	if got := report.Summary.UpdatedRepoCount; got != 1 {
		t.Fatalf("unexpected updated repo count: %d", got)
	}
	if !strings.Contains(report.PrimaryNextAction, "/tmp/bin") {
		t.Fatalf("expected shell reload hint to name managed bin dir, got %q", report.PrimaryNextAction)
	}
	if got := report.ExitCode(); got != 0 {
		t.Fatalf("unexpected exit code: %d", got)
	}
}

func TestBuildSyncReportCompletedWithBlockersCountsTerminalReadyRepoBlockers(t *testing.T) {
	managedBinDir := filepath.Join("/tmp", "bin")
	finalStatus := StatusReport{
		Schema:        StatusReportSchema,
		ManagedBinDir: managedBinDir,
		Summary: StatusSummary{
			Overall:     StateShadowed,
			RepoCount:   1,
			StateCounts: map[string]int{StateShadowed: 1},
		},
		Repos: []RepoStatus{
			{
				RepoID:        "keystone-hub",
				AdapterStatus: AdapterStatusReady,
				State:         StateShadowed,
				Outputs: []OutputStatus{
					{
						Name:         "kshub",
						ExpectedPath: filepath.Join(managedBinDir, "kshub"),
						ResolvedPath: "/usr/local/bin/kshub",
						State:        StateShadowed,
						Reason:       "PATH resolves /usr/local/bin/kshub instead of /tmp/bin/kshub",
					},
				},
			},
		},
	}

	report := BuildSyncReport(PersistedState{}, PersistedState{}, finalStatus)
	if got := report.Outcome; got != SyncOutcomeCompletedWithBlockers {
		t.Fatalf("unexpected outcome: %s", got)
	}
	if got := report.Summary.BlockedRepoCount; got != 1 {
		t.Fatalf("unexpected blocked repo count: %d", got)
	}
	if got := report.Summary.ShadowedOutputCount; got != 1 {
		t.Fatalf("unexpected shadowed output count: %d", got)
	}
	if got := report.ExitCode(); got != contract.ExitReadySetBlocked {
		t.Fatalf("unexpected exit code: got=%d want=%d", got, contract.ExitReadySetBlocked)
	}
	if report.EnvelopeOK() {
		t.Fatal("completed_with_blockers must be a result-bearing non-success")
	}
	if !strings.Contains(report.PrimaryNextAction, "PATH") {
		t.Fatalf("expected PATH repair next action, got %q", report.PrimaryNextAction)
	}
}

func TestRenderSyncTextDefaultAndVerboseDiffer(t *testing.T) {
	report := SyncReport{
		Schema:  SyncReportSchema,
		Outcome: SyncOutcomeCompletedWithBlockers,
		Summary: SyncSummary{
			ReadyRepoCount:      1,
			BlockedRepoCount:    1,
			ShadowedOutputCount: 1,
		},
		PrimaryNextAction: "put /tmp/bin first on PATH, then open a new shell",
		FinalStatus: StatusReport{
			Schema:        StatusReportSchema,
			ManagedBinDir: "/tmp/bin",
			Summary: StatusSummary{
				Overall:     StateShadowed,
				RepoCount:   1,
				StateCounts: map[string]int{StateShadowed: 1},
			},
			Repos: []RepoStatus{
				{
					RepoID:        "keystone-hub",
					AdapterStatus: AdapterStatusReady,
					State:         StateShadowed,
					Reason:        "PATH resolves /usr/local/bin/kshub instead of /tmp/bin/kshub",
					RepoPath:      "/tmp/hub",
					Outputs: []OutputStatus{
						{Name: "kshub", ExpectedPath: "/tmp/bin/kshub", ResolvedPath: "/usr/local/bin/kshub", State: StateShadowed},
					},
				},
			},
		},
	}

	defaultText := strings.Join(RenderSyncText(report, false), "\n")
	verboseText := strings.Join(RenderSyncText(report, true), "\n")
	if strings.Contains(defaultText, "Suite:") {
		t.Fatalf("default sync text must not collapse into the broad status dump: %s", defaultText)
	}
	if !strings.Contains(verboseText, "Suite:") {
		t.Fatalf("verbose sync text should widen to final status detail: %s", verboseText)
	}
}

func TestBuildSyncReportTreatsAttemptIntegrityReducedFinalStatusAsBlocked(t *testing.T) {
	report := BuildSyncReport(
		PersistedState{},
		PersistedState{},
		StatusReport{
			Schema:        StatusReportSchema,
			ManagedBinDir: "/tmp/bin",
			Summary: StatusSummary{
				Overall:     StateUnknown,
				RepoCount:   1,
				StateCounts: map[string]int{StateCurrent: 1},
			},
			AttemptIntegrity: &AttemptIntegrityStatus{
				State: AttemptIntegrityPromotionLate,
			},
			Repos: []RepoStatus{
				{
					RepoID:        "keystone-hub",
					AdapterStatus: AdapterStatusReady,
					State:         StateCurrent,
					Outputs: []OutputStatus{
						{Name: "kshub", ExpectedPath: "/tmp/bin/kshub", State: StateCurrent},
					},
				},
			},
		},
	)
	if got := report.Outcome; got != SyncOutcomeCompletedWithBlockers {
		t.Fatalf("attempt-integrity-reduced final status must be non-success, got %s", got)
	}
}
