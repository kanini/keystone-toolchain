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

func SyncReadySet(ctx *runtime.Context, manifest Manifest, prior PersistedState, priorPresent bool) *contract.AppError {
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
		repoState := syncReadyAdapter(ctx, adapter, priorIndex[adapter.RepoID])
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

func syncReadyAdapter(ctx *runtime.Context, adapter RepoAdapter, prior PersistedRepoState) PersistedRepoState {
	repoState := PersistedRepoState{
		RepoID:  adapter.RepoID,
		Outputs: append([]string{}, adapter.ExpectedOutputs...),
	}

	if repoHead, ok := lookupRepoHead(adapter.RepoPath); ok {
		repoState.RepoHead = repoHead
	}

	dirty, reason := repoDirty(adapter.RepoPath)
	if dirty {
		repoState.State = StateDirtySkipped
		repoState.Reason = reason
		repoState.ActiveBuild = prior.ActiveBuild
		return repoState
	}
	if reason != "" {
		applyFailureState(&repoState, prior, truncateReason(reason))
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

	for _, output := range adapter.ExpectedOutputs {
		if _, err := os.Stat(filepath.Join(stageBin, output)); err != nil {
			applyFailureState(&repoState, prior, truncateReason(fmt.Sprintf("expected staged output %s is missing", output)))
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
	for _, output := range adapter.ExpectedOutputs {
		source := filepath.Join(stageBin, output)
		target := filepath.Join(ctx.Config.ManagedBinDir, output)
		if err := promoteFile(source, target, os.Rename); err != nil {
			applyFailureState(&repoState, prior, truncateReason(fmt.Sprintf("could not promote %s: %v", output, err)))
			return repoState
		}
	}

	repoState.State = StateCurrent
	repoState.ActiveBuild = repoState.RepoHead
	return repoState
}

func repoDirty(repoPath string) (bool, string) {
	cmd := exec.Command("git", "-C", repoPath, "status", "--porcelain")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Sprintf("could not inspect repo dirtiness: %v", err)
	}
	if strings.TrimSpace(string(out)) != "" {
		return true, "repo has uncommitted changes; sync is fail_closed in v1"
	}
	return false, ""
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
		repoState.ActiveBuild = prior.ActiveBuild
	} else {
		repoState.State = StateFailed
	}
	repoState.Reason = reason
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

func promoteFile(source, target string, renameFn func(string, string) error) error {
	if err := renameFn(source, target); err == nil {
		return nil
	} else if !errors.Is(err, syscall.EXDEV) {
		return err
	}

	info, err := os.Stat(source)
	if err != nil {
		return err
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
	if err := os.Remove(source); err != nil {
		return err
	}
	cleanup = false
	return nil
}
