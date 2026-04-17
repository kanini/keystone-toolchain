package toolchain

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kanini/keystone-toolchain/internal/contract"
	"github.com/kanini/keystone-toolchain/internal/runtime"
)

const InitReportSchema = "kstoolchain.init/v1alpha1"

type InitOptions struct {
	DryRun bool
	Shell  string
}

type InitReport struct {
	Schema         string        `json:"schema"`
	OverlayPath    string        `json:"overlay_path"`
	DryRun         bool          `json:"dry_run"`
	Changed        bool          `json:"changed"`
	Applied        bool          `json:"applied"`
	Bootstrap      InitBootstrap `json:"bootstrap"`
	Repos          []InitRepo    `json:"repos"`
	UnknownRows    []OverlayRepo `json:"unknown_rows,omitempty"`
	Diff           []OverlayDiff `json:"diff,omitempty"`
	Delegated      bool          `json:"delegated"`
	ReadySet       *StatusReport `json:"ready_set,omitempty"`
	ChangedItems   []string      `json:"changed_items,omitempty"`
	AlreadyCorrect []string      `json:"already_correct,omitempty"`
	ManualActions  []string      `json:"manual_actions,omitempty"`

	delegateReadySet bool
	exitCode         int
}

type InitRepo struct {
	RepoID   string `json:"repo_id"`
	RepoPath string `json:"repo_path,omitempty"`
	Source   string `json:"source"`
	Setup    string `json:"setup,omitempty"`
}

type OverlayDiff struct {
	Change  string `json:"change"`
	RepoID  string `json:"repo_id"`
	OldPath string `json:"old_path,omitempty"`
	NewPath string `json:"new_path,omitempty"`
}

func RunInitFlow(ctx *runtime.Context, in io.Reader, out io.Writer, opts InitOptions) (InitReport, *contract.AppError) {
	overlayPath, _, appErr := resolveOverlayPath(ctx)
	if appErr != nil {
		return InitReport{}, appErr
	}

	report := InitReport{
		Schema:      InitReportSchema,
		OverlayPath: overlayPath,
		DryRun:      opts.DryRun,
		exitCode:    contract.ExitOK,
	}
	if appErr := applyInitBootstrap(&report, ctx, opts, overlayPath); appErr != nil {
		return InitReport{}, appErr
	}

	manifest, knownRepoIDs, appErr := loadTrackedManifest()
	if appErr != nil {
		return InitReport{}, appErr
	}
	overlay, appErr := loadOverlaySelection(ctx, knownRepoIDs, overlayLoadOptions{
		allowMissing:       true,
		emitMissingWarning: false,
	})
	if appErr != nil {
		return InitReport{}, appErr
	}

	report.OverlayPath = overlay.Path
	sources := map[string]string{}
	proposed := map[string]string{}
	roots := likelyRepoRoots(ctx)

	for _, adapter := range manifest.Repos {
		repoID := adapter.RepoID
		existingPath, hasExisting := overlay.Entries[repoID]
		switch {
		case hasExisting:
			proposed[repoID] = existingPath
			sources[repoID] = "existing"
			if setup := classifyRepoSetup(existingPath); setup.Reason == SetupReasonRepoPathMissing {
				if discovered, ok := discoverRepoPath(repoID, roots); ok && discovered != existingPath {
					proposed[repoID] = discovered
					sources[repoID] = "discovered"
				}
			}
		default:
			if discovered, ok := discoverRepoPath(repoID, roots); ok {
				proposed[repoID] = discovered
				sources[repoID] = "discovered"
			} else {
				sources[repoID] = "unset"
			}
		}
	}

	report.UnknownRows = append(report.UnknownRows, overlay.UnknownRows...)
	renderBootstrapReview(out, report)
	refreshInitReport(&report, manifest.Repos, proposed, sources)
	renderInitReview(out, report)

	if opts.DryRun {
		finalRows := buildOverlayRows(report.Repos, report.UnknownRows)
		report.Diff = semanticOverlayDiff(buildOverlayRowsFromSelection(overlay), finalRows)
		renderOverlayDiff(out, report.Diff)
		report.Changed = len(report.ChangedItems) > 0 || len(report.Diff) > 0
		report.delegateReadySet = false
		return report, nil
	}

	reader := bufio.NewReader(in)
	if appErr := promptRepoCorrections(reader, out, manifest.Repos, &report, proposed, sources, ctx.HomeDir); appErr != nil {
		return InitReport{}, appErr
	}
	if appErr := promptUnknownRowDecisions(reader, out, &report); appErr != nil {
		return InitReport{}, appErr
	}

	finalRows := buildOverlayRows(report.Repos, report.UnknownRows)
	report.Diff = semanticOverlayDiff(buildOverlayRowsFromSelection(overlay), finalRows)
	renderOverlayDiff(out, report.Diff)
	report.Changed = len(report.ChangedItems) > 0 || len(report.Diff) > 0
	if len(report.Diff) == 0 {
		report.delegateReadySet = report.exitCode == contract.ExitOK
		return report, nil
	}

	approved, appErr := promptApproval(reader, out)
	if appErr != nil {
		return InitReport{}, appErr
	}
	if !approved {
		report.ManualActions = appendUniqueStrings(report.ManualActions, fmt.Sprintf("approve or apply the overlay changes in %s before relying on the ready set, then run `kstoolchain init` again", report.OverlayPath))
		report.delegateReadySet = false
		report.exitCode = contract.ExitValidation
		return report, nil
	}

	if appErr := writeOverlayFile(report.OverlayPath, finalRows); appErr != nil {
		return InitReport{}, appErr
	}
	report.Applied = true
	report.delegateReadySet = report.exitCode == contract.ExitOK
	report.ChangedItems = append(report.ChangedItems, fmt.Sprintf("wrote adapters overlay %s", report.OverlayPath))
	return report, nil
}

func (report InitReport) ShouldDelegateReadySet() bool {
	return report.delegateReadySet
}

func (report InitReport) ExitCode() int {
	if report.exitCode == 0 {
		return contract.ExitOK
	}
	return report.exitCode
}

func likelyRepoRoots(ctx *runtime.Context) []string {
	roots := []string{}
	seen := map[string]struct{}{}
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		path = filepath.Clean(path)
		if _, ok := seen[path]; ok {
			return
		}
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			seen[path] = struct{}{}
			roots = append(roots, path)
		}
	}

	add(filepath.Join(ctx.HomeDir, "git"))
	if wd, err := os.Getwd(); err == nil {
		add(filepath.Dir(wd))
		add(wd)
	}
	add(filepath.Join(ctx.HomeDir, "src"))
	return roots
}

func discoverRepoPath(repoID string, roots []string) (string, bool) {
	for _, root := range roots {
		candidate := filepath.Join(root, repoID)
		if setup := classifyRepoSetup(candidate); setup.State != StateSetupBlocked {
			return candidate, true
		}
	}
	return "", false
}

func refreshInitReport(report *InitReport, adapters []RepoAdapter, proposed map[string]string, sources map[string]string) {
	report.Repos = report.Repos[:0]
	for _, adapter := range adapters {
		repoPath := proposed[adapter.RepoID]
		setup := classifyRepoSetup(repoPath)
		initRepo := InitRepo{
			RepoID:   adapter.RepoID,
			RepoPath: repoPath,
			Source:   sources[adapter.RepoID],
		}
		if initRepo.Source == "" {
			initRepo.Source = "unset"
		}
		if setup.Reason != "" {
			initRepo.Setup = setup.Reason
		}
		report.Repos = append(report.Repos, initRepo)
	}
}

func renderBootstrapReview(out io.Writer, report InitReport) {
	fmt.Fprintln(out, "Bootstrap:")
	fmt.Fprintf(out, "MANAGED_BIN  %s\n", report.Bootstrap.ManagedBinDir)
	fmt.Fprintf(out, "STATE_DIR    %s\n", report.Bootstrap.StateDir)
	if report.Bootstrap.Shell != "" {
		fmt.Fprintf(out, "SHELL        %s\n", report.Bootstrap.Shell)
	}
	if report.Bootstrap.RCPath != "" {
		fmt.Fprintf(out, "RC_FILE      %s  (%s)\n", report.Bootstrap.RCPath, report.Bootstrap.RCStatus)
	}
	fmt.Fprintln(out, "DIRS:")
	for _, dir := range report.Bootstrap.Dirs {
		fmt.Fprintf(out, "- %s  %s\n", dir.Status, dir.Path)
	}
	fmt.Fprintln(out, "")
}

func renderInitReview(out io.Writer, report InitReport) {
	fmt.Fprintf(out, "Overlay target: %s\n", report.OverlayPath)
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Tracked repos:")
	fmt.Fprintln(out, "REPO_ID             SOURCE      SETUP               REPO_PATH")
	for _, repo := range report.Repos {
		repoPath := repo.RepoPath
		if repoPath == "" {
			repoPath = "-"
		}
		setup := repo.Setup
		if setup == "" {
			setup = "ok"
		}
		fmt.Fprintf(out, "%-19s %-11s %-19s %s\n", repo.RepoID, repo.Source, setup, repoPath)
	}
	if len(report.UnknownRows) > 0 {
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "Unknown overlay rows:")
		for _, row := range report.UnknownRows {
			fmt.Fprintf(out, "- %s -> %s\n", row.RepoID, row.RepoPath)
		}
	}
}

func RenderInitText(report InitReport) []string {
	lines := []string{}
	switch {
	case report.DryRun:
		lines = append(lines, "Init: preview only")
	case report.Delegated && report.ReadySet != nil:
		lines = append(lines, fmt.Sprintf("Init: delegated ready-set result %s", report.ReadySet.Summary.Overall))
	case report.ExitCode() == contract.ExitOK:
		lines = append(lines, "Init: bootstrap and overlay setup complete")
	default:
		lines = append(lines, "Init: non-success")
	}

	lines = append(lines,
		fmt.Sprintf("Managed bin: %s", report.Bootstrap.ManagedBinDir),
		fmt.Sprintf("State dir: %s", report.Bootstrap.StateDir),
		fmt.Sprintf("Overlay: %s", report.OverlayPath),
	)
	if report.Bootstrap.Shell != "" {
		shellLine := fmt.Sprintf("Shell: %s", report.Bootstrap.Shell)
		if report.Bootstrap.RCPath != "" {
			shellLine = fmt.Sprintf("%s  rc=%s  status=%s", shellLine, report.Bootstrap.RCPath, report.Bootstrap.RCStatus)
		}
		lines = append(lines, shellLine)
	}
	if report.DryRun {
		lines = append(lines, "Preview only. No bootstrap files, overlay files, or ready-set state were written.")
	}
	if len(report.ChangedItems) > 0 {
		lines = append(lines, "Changed:")
		for _, item := range report.ChangedItems {
			lines = append(lines, fmt.Sprintf("  - %s", item))
		}
	}
	if len(report.AlreadyCorrect) > 0 {
		lines = append(lines, "Already correct:")
		for _, item := range report.AlreadyCorrect {
			lines = append(lines, fmt.Sprintf("  - %s", item))
		}
	}
	if report.Delegated && report.ReadySet != nil {
		lines = append(lines, "Delegated ready-set result:")
		for _, line := range RenderStatusText(*report.ReadySet) {
			lines = append(lines, "  "+line)
		}
	} else if !report.DryRun {
		lines = append(lines, "Delegated ready-set result: skipped")
	}
	if len(report.ManualActions) > 0 {
		lines = append(lines, "Manual actions:")
		for _, action := range report.ManualActions {
			lines = append(lines, fmt.Sprintf("  - %s", action))
		}
	}
	return lines
}

func appendUniqueStrings(dst []string, items ...string) []string {
	seen := map[string]struct{}{}
	for _, item := range dst {
		seen[item] = struct{}{}
	}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		dst = append(dst, item)
	}
	return dst
}

func promptRepoCorrections(reader *bufio.Reader, out io.Writer, adapters []RepoAdapter, report *InitReport, proposed map[string]string, sources map[string]string, home string) *contract.AppError {
	valid := map[string]struct{}{}
	for _, adapter := range adapters {
		valid[adapter.RepoID] = struct{}{}
	}

	fmt.Fprintln(out, "")
	fmt.Fprint(out, "Edit repo paths before write? Enter comma-separated repo_ids, or press Enter to continue: ")
	line, err := readPromptLine(reader)
	if err != nil {
		return err
	}
	if strings.TrimSpace(line) == "" {
		return nil
	}

	for _, item := range strings.Split(line, ",") {
		repoID := strings.TrimSpace(item)
		if repoID == "" {
			continue
		}
		if _, ok := valid[repoID]; !ok {
			return contract.ArgsInvalid(fmt.Sprintf("Unknown repo_id %s.", repoID), "Choose repo_ids from the tracked review table.")
		}
		current := proposed[repoID]
		if current == "" {
			current = "-"
		}
		fmt.Fprintf(out, "repo_path for %s [%s] (blank to unset): ", repoID, current)
		rawPath, appErr := readPromptLine(reader)
		if appErr != nil {
			return appErr
		}
		rawPath = strings.TrimSpace(rawPath)
		if rawPath == "" {
			delete(proposed, repoID)
			sources[repoID] = "edited"
			continue
		}
		normalized, err := runtime.NormalizePath(rawPath, home)
		if err != nil || strings.TrimSpace(normalized) == "" {
			return contract.Validation(contract.CodeOverlayInvalid, fmt.Sprintf("repo_path for %s is invalid.", repoID), "Enter an absolute path or a ~/ path.")
		}
		proposed[repoID] = normalized
		sources[repoID] = "edited"
	}

	refreshInitReport(report, adapters, proposed, sources)
	return nil
}

func promptUnknownRowDecisions(reader *bufio.Reader, out io.Writer, report *InitReport) *contract.AppError {
	if len(report.UnknownRows) == 0 {
		return nil
	}
	fmt.Fprintln(out, "")
	kept := make([]OverlayRepo, 0, len(report.UnknownRows))
	for _, row := range report.UnknownRows {
		for {
			fmt.Fprintf(out, "Unknown row %s -> %s. Keep or remove? [k/r]: ", row.RepoID, row.RepoPath)
			answer, appErr := readPromptLine(reader)
			if appErr != nil {
				return appErr
			}
			switch strings.ToLower(strings.TrimSpace(answer)) {
			case "k", "keep":
				kept = append(kept, row)
				goto nextRow
			case "r", "remove":
				goto nextRow
			default:
				fmt.Fprintln(out, "Enter k to keep or r to remove.")
			}
		}
	nextRow:
	}
	report.UnknownRows = kept
	return nil
}

func promptApproval(reader *bufio.Reader, out io.Writer) (bool, *contract.AppError) {
	fmt.Fprintln(out, "")
	fmt.Fprint(out, "Write overlay file? [y/N]: ")
	answer, appErr := readPromptLine(reader)
	if appErr != nil {
		if appErr.Code == contract.CodeArgsInvalid {
			return false, nil
		}
		return false, appErr
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

func readPromptLine(reader *bufio.Reader) (string, *contract.AppError) {
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", contract.Infra(contract.CodeIOError, "Could not read init input.", "Retry the command.", err)
	}
	if errors.Is(err, io.EOF) && line == "" {
		return "", contract.ArgsInvalid("Interactive input ended before the init flow completed.", "Run `kstoolchain init` in an interactive shell or use --dry-run.")
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func buildOverlayRows(repos []InitRepo, unknownRows []OverlayRepo) []OverlayRepo {
	rows := make([]OverlayRepo, 0, len(repos)+len(unknownRows))
	for _, repo := range repos {
		if strings.TrimSpace(repo.RepoPath) == "" {
			continue
		}
		rows = append(rows, OverlayRepo{RepoID: repo.RepoID, RepoPath: repo.RepoPath})
	}
	rows = append(rows, unknownRows...)
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].RepoID < rows[j].RepoID
	})
	return rows
}

func buildOverlayRowsFromSelection(overlay OverlaySelection) []OverlayRepo {
	rows := make([]OverlayRepo, 0, len(overlay.Entries)+len(overlay.UnknownRows))
	for repoID, repoPath := range overlay.Entries {
		rows = append(rows, OverlayRepo{RepoID: repoID, RepoPath: repoPath})
	}
	rows = append(rows, overlay.UnknownRows...)
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].RepoID < rows[j].RepoID
	})
	return rows
}

func semanticOverlayDiff(before, after []OverlayRepo) []OverlayDiff {
	beforeMap := map[string]string{}
	afterMap := map[string]string{}
	ids := map[string]struct{}{}
	for _, row := range before {
		beforeMap[row.RepoID] = row.RepoPath
		ids[row.RepoID] = struct{}{}
	}
	for _, row := range after {
		afterMap[row.RepoID] = row.RepoPath
		ids[row.RepoID] = struct{}{}
	}

	keys := make([]string, 0, len(ids))
	for repoID := range ids {
		keys = append(keys, repoID)
	}
	sort.Strings(keys)

	diff := make([]OverlayDiff, 0, len(keys))
	for _, repoID := range keys {
		beforePath, hadBefore := beforeMap[repoID]
		afterPath, hasAfter := afterMap[repoID]
		switch {
		case !hadBefore && hasAfter:
			diff = append(diff, OverlayDiff{Change: "add", RepoID: repoID, NewPath: afterPath})
		case hadBefore && !hasAfter:
			diff = append(diff, OverlayDiff{Change: "remove", RepoID: repoID, OldPath: beforePath})
		case hadBefore && hasAfter && beforePath != afterPath:
			diff = append(diff, OverlayDiff{Change: "change", RepoID: repoID, OldPath: beforePath, NewPath: afterPath})
		}
	}
	return diff
}

func renderOverlayDiff(out io.Writer, diff []OverlayDiff) {
	fmt.Fprintln(out, "")
	if len(diff) == 0 {
		fmt.Fprintln(out, "Semantic diff: no changes.")
		return
	}
	fmt.Fprintln(out, "Semantic diff:")
	for _, item := range diff {
		switch item.Change {
		case "add":
			fmt.Fprintf(out, "+ %s -> %s\n", item.RepoID, item.NewPath)
		case "remove":
			fmt.Fprintf(out, "- %s (was %s)\n", item.RepoID, item.OldPath)
		case "change":
			fmt.Fprintf(out, "~ %s: %s -> %s\n", item.RepoID, item.OldPath, item.NewPath)
		}
	}
}

func writeOverlayFile(path string, rows []OverlayRepo) *contract.AppError {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return contract.Infra(contract.CodeIOError, "Could not create adapters overlay directory.", "Check filesystem permissions and retry.", err, contract.Detail{Name: "path", Value: filepath.Dir(path)})
	}

	data, appErr := encodeOverlayDocument(rows)
	if appErr != nil {
		return appErr
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(path), "adapters-*.tmp")
	if err != nil {
		return contract.Infra(contract.CodeIOError, "Could not create temp adapters overlay file.", "Check filesystem permissions and retry.", err, contract.Detail{Name: "path", Value: filepath.Dir(path)})
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return contract.Infra(contract.CodeIOError, "Could not write temp adapters overlay file.", "Retry after checking filesystem health.", err, contract.Detail{Name: "path", Value: tmpPath})
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return contract.Infra(contract.CodeIOError, "Could not close temp adapters overlay file.", "Retry after checking filesystem health.", err, contract.Detail{Name: "path", Value: tmpPath})
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return contract.Infra(contract.CodeIOError, "Could not atomically write adapters overlay file.", "Check that the destination directory is writable and retry.", err, contract.Detail{Name: "path", Value: path})
	}
	return nil
}
