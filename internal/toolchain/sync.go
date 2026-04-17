package toolchain

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/kanini/keystone-toolchain/internal/contract"
	"github.com/kanini/keystone-toolchain/internal/runtime"
)

type SyncOptions struct {
	AllowDirty bool
}

type repoObservation struct {
	Head       string
	SourceKind string
}

func SyncReadySet(ctx *runtime.Context, manifest Manifest, prior PersistedState, priorPresent bool, opts SyncOptions) *contract.AppError {
	ready := SelectReadyAdapters(manifest)
	if len(ready) == 0 {
		return contract.Validation(contract.CodeConfigInvalid, "Adapter manifest does not declare any ready adapters.", "Mark at least one adapter ready before running sync.")
	}

	now := ctx.Now().UTC().Format("2006-01-02T15:04:05Z")
	next := PersistedState{
		Schema:        PersistedStateSchema,
		ManagedBinDir: ctx.Config.ManagedBinDir,
		LastAttemptAt: now,
		LastSuccessAt: prior.LastSuccessAt,
		Repos:         make([]PersistedRepoState, 0, len(ready)),
	}

	priorIndex := map[string]PersistedRepoState{}
	if priorPresent {
		for _, repo := range prior.Repos {
			priorIndex[repo.RepoID] = repo
		}
	}

	allSucceeded := true
	for _, adapter := range ready {
		repoState := syncReadyAdapter(ctx, adapter, priorIndex[adapter.RepoID], opts)
		if repoState.State != StateCurrent {
			allSucceeded = false
		}
		next.Repos = append(next.Repos, repoState)
	}
	if allSucceeded {
		next.LastSuccessAt = now
	}

	_, appErr := SavePersistedState(ctx, next)
	return appErr
}

func syncReadyAdapter(ctx *runtime.Context, adapter RepoAdapter, prior PersistedRepoState, opts SyncOptions) PersistedRepoState {
	repoState := PersistedRepoState{
		RepoID:  adapter.RepoID,
		Outputs: append([]string{}, adapter.ExpectedOutputs...),
	}

	setup := classifyRepoSetup(adapter.RepoPath)
	if setup.State == StateSetupBlocked {
		repoState.State = StateSetupBlocked
		repoState.Reason = setup.Reason
		copyClassifiedInputPair(&repoState, prior)
		copyActiveBuildPair(&repoState, prior)
		return repoState
	}

	classified, reason := observeRepoSource(adapter.RepoPath)
	if reason != "" {
		copyClassifiedInputPair(&repoState, prior)
		applyFailureState(&repoState, prior, truncateReason(reason))
		return repoState
	}
	setClassifiedInputPair(&repoState, classified.Head, classified.SourceKind)
	if classified.SourceKind == SourceKindDirtyWorktree && !opts.AllowDirty {
		repoState.State = StateDirtySkipped
		repoState.Reason = dirtyRepoReason()
		copyActiveBuildPair(&repoState, prior)
		return repoState
	}

	stageRoot := filepath.Join(ctx.Config.StateDir, "staging", fmt.Sprintf("%s-%s", adapter.RepoID, ctx.Now().UTC().Format("20060102T150405.000000000Z")))
	stageBin := filepath.Join(stageRoot, "bin")
	if err := os.MkdirAll(stageBin, 0o755); err != nil {
		applyFailureState(&repoState, prior, truncateReason(fmt.Sprintf("could not create stage dir: %v", err)))
		return repoState
	}
	defer os.RemoveAll(stageRoot)

	vars := templateVars{
		stageBin:       stageBin,
		stageBinParent: stageRoot,
	}

	installCmd, appErr := expandCommandArgs(adapter.InstallCmd, vars)
	if appErr != nil {
		applyFailureState(&repoState, prior, appErr.Message)
		return repoState
	}
	if output, err := runRepoCommand(adapter.RepoPath, installCmd, adapter.Env); err != nil {
		applyFailureState(&repoState, prior, commandFailureReason("install", output, err))
		return repoState
	}

	artifacts := promotedArtifacts(adapter)
	for _, artifact := range artifacts {
		if _, err := os.Stat(filepath.Join(stageBin, artifact)); err != nil {
			applyFailureState(&repoState, prior, truncateReason(fmt.Sprintf("expected staged artifact %s is missing", artifact)))
			return repoState
		}
	}

	probeCmd, appErr := expandCommandArgs(adapter.ProbeCmd, vars)
	if appErr != nil {
		applyFailureState(&repoState, prior, appErr.Message)
		return repoState
	}
	if output, err := runRepoCommand(adapter.RepoPath, probeCmd, adapter.Env); err != nil {
		applyFailureState(&repoState, prior, commandFailureReason("probe", output, err))
		return repoState
	}

	if err := os.MkdirAll(ctx.Config.ManagedBinDir, 0o755); err != nil {
		applyFailureState(&repoState, prior, truncateReason(fmt.Sprintf("could not create managed bin dir: %v", err)))
		return repoState
	}

	promotionObservation, reason := observeRepoSource(adapter.RepoPath)
	if reason != "" {
		applyFailureState(&repoState, prior, truncateReason(reason))
		return repoState
	}
	if reason := validatePromotionBoundary(classified, promotionObservation); reason != "" {
		applyFailureState(&repoState, prior, reason)
		return repoState
	}
	for _, artifact := range artifacts {
		source := filepath.Join(stageBin, artifact)
		target := filepath.Join(ctx.Config.ManagedBinDir, artifact)
		if err := promotePath(source, target, os.Rename); err != nil {
			applyFailureState(&repoState, prior, truncateReason(fmt.Sprintf("could not promote %s: %v", artifact, err)))
			return repoState
		}
	}

	repoState.State = StateCurrent
	setActiveBuildPair(&repoState, classified.Head, classified.SourceKind)
	return repoState
}

func observeRepoSource(repoPath string) (repoObservation, string) {
	gitPath, ok := resolveGitBinary()
	if !ok {
		return repoObservation{}, "could not inspect repo dirtiness: git executable not found"
	}
	cmd := exec.Command(gitPath, "-C", repoPath, "status", "--porcelain=v2", "--branch")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return repoObservation{}, fmt.Sprintf("could not inspect repo dirtiness: %v", err)
	}
	observation, err := parseRepoObservation(string(out))
	if err != nil {
		return repoObservation{}, truncateReason(fmt.Sprintf("could not inspect repo dirtiness: %v", err))
	}
	return observation, ""
}

func runRepoCommand(repoPath string, args []string, env map[string]string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("empty command")
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = repoPath
	if len(env) > 0 {
		cmd.Env = os.Environ()
		for key, value := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
		}
	}
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func applyFailureState(repoState *PersistedRepoState, prior PersistedRepoState, reason string) {
	if prior.ActiveBuild != "" {
		repoState.State = StateStaleLKG
		copyActiveBuildPair(repoState, prior)
	} else {
		repoState.State = StateFailed
	}
	repoState.Reason = reason
}

func dirtyRepoReason() string {
	return "repo has uncommitted changes; sync is fail_closed in v1"
}

func copyClassifiedInputPair(dst *PersistedRepoState, src PersistedRepoState) {
	dst.RepoHead = src.RepoHead
	dst.LastAttemptSourceKind = src.LastAttemptSourceKind
}

func copyActiveBuildPair(dst *PersistedRepoState, src PersistedRepoState) {
	dst.ActiveBuild = src.ActiveBuild
	dst.ActiveSourceKind = src.ActiveSourceKind
}

func setClassifiedInputPair(dst *PersistedRepoState, head, sourceKind string) {
	dst.RepoHead = strings.TrimSpace(head)
	dst.LastAttemptSourceKind = strings.TrimSpace(sourceKind)
}

func setActiveBuildPair(dst *PersistedRepoState, head, sourceKind string) {
	dst.ActiveBuild = strings.TrimSpace(head)
	dst.ActiveSourceKind = strings.TrimSpace(sourceKind)
}

func parseRepoObservation(raw string) (repoObservation, error) {
	observation := repoObservation{}
	dirty := false
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "# branch.oid ") {
			observation.Head = strings.TrimSpace(strings.TrimPrefix(line, "# branch.oid "))
			continue
		}
		if strings.HasPrefix(line, "# ") {
			continue
		}
		dirty = true
	}
	if observation.Head == "" || observation.Head == "(initial)" || observation.Head == "(unknown)" {
		return repoObservation{}, fmt.Errorf("repo HEAD could not be resolved")
	}
	if dirty {
		observation.SourceKind = SourceKindDirtyWorktree
	} else {
		observation.SourceKind = SourceKindCleanHead
	}
	return observation, nil
}

func validatePromotionBoundary(classified, promotion repoObservation) string {
	if promotion.Head != classified.Head {
		return truncateReason(fmt.Sprintf("repo HEAD changed from %s to %s before promotion boundary", shortValue(classified.Head), shortValue(promotion.Head)))
	}
	if classified.SourceKind == SourceKindCleanHead && promotion.SourceKind != SourceKindCleanHead {
		return "repo became dirty before promotion boundary"
	}
	return ""
}

func commandFailureReason(step, output string, err error) string {
	msg := fmt.Sprintf("%s command could not run: %v", step, err)
	if strings.TrimSpace(output) != "" {
		msg = fmt.Sprintf("%s | %s", msg, output)
	}
	return truncateReason(msg)
}

func truncateReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if len(reason) <= 200 {
		return reason
	}
	return strings.TrimSpace(reason[:197]) + "..."
}

func promotePath(source, target string, renameFn func(string, string) error) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	if err := renameFn(source, target); err == nil {
		return nil
	} else if !errors.Is(err, syscall.EXDEV) {
		if !info.IsDir() || !existingNonEmptyTarget(target) {
			return err
		}
		return promoteDirectory(source, target, info.Mode(), renameFn)
	}

	if info.IsDir() {
		return promoteDirectory(source, target, info.Mode(), renameFn)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(target), filepath.Base(target)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	sourceFile, err := os.Open(source)
	if err != nil {
		_ = tmpFile.Close()
		return err
	}
	if _, err := io.Copy(tmpFile, sourceFile); err != nil {
		_ = sourceFile.Close()
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		_ = sourceFile.Close()
		_ = tmpFile.Close()
		return err
	}
	if err := sourceFile.Close(); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, info.Mode()); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, target); err != nil {
		return err
	}
	_ = os.Remove(source)
	cleanup = false
	return nil
}

func promoteDirectory(source, target string, mode os.FileMode, renameFn func(string, string) error) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	targetExists := false
	if _, err := os.Stat(target); err == nil {
		targetExists = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if !targetExists {
		if err := renameFn(source, target); err == nil {
			return nil
		} else if !errors.Is(err, syscall.EXDEV) {
			return err
		}
	}

	tmpDir, err := os.MkdirTemp(filepath.Dir(target), filepath.Base(target)+".tmp-*")
	if err != nil {
		return err
	}
	cleanupTmp := true
	defer func() {
		if cleanupTmp {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	if err := os.Chmod(tmpDir, mode); err != nil {
		return err
	}
	if err := copyDirectory(source, tmpDir); err != nil {
		return err
	}

	backupPath := ""
	if targetExists {
		backupPath = filepath.Join(filepath.Dir(target), filepath.Base(target)+".bak-"+filepath.Base(tmpDir))
		if err := os.RemoveAll(backupPath); err != nil {
			return err
		}
		if err := renameFn(target, backupPath); err != nil {
			return err
		}
		defer func() {
			if backupPath != "" {
				_ = renameFn(backupPath, target)
			}
		}()
	}

	if err := renameFn(tmpDir, target); err != nil {
		return err
	}
	committedBackup := backupPath
	backupPath = ""
	cleanupTmp = false
	_ = os.RemoveAll(source)
	if committedBackup != "" {
		_ = os.RemoveAll(committedBackup)
	}
	return nil
}

func existingNonEmptyTarget(target string) bool {
	info, err := os.Stat(target)
	if err != nil || !info.IsDir() {
		return false
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return false
	}
	return len(entries) > 0
}

func copyDirectory(source, target string) error {
	return filepath.Walk(source, func(current string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, current)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		destination := filepath.Join(target, rel)
		if info.IsDir() {
			return os.MkdirAll(destination, info.Mode())
		}

		sourceFile, err := os.Open(current)
		if err != nil {
			return err
		}
		defer sourceFile.Close()

		targetFile, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		if _, err := io.Copy(targetFile, sourceFile); err != nil {
			_ = targetFile.Close()
			return err
		}
		if err := targetFile.Close(); err != nil {
			return err
		}
		return nil
	})
}
