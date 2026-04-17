package toolchain

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	StateSetupBlocked = "SETUP_BLOCKED"

	SetupReasonRepoPathUnset      = "repo_path_unset"
	SetupReasonRepoPathMissing    = "repo_path_missing"
	SetupReasonRepoPathNotGit     = "repo_path_not_git"
	SetupReasonRepoPathUnreadable = "repo_path_unreadable"
	SetupReasonRepoPathInvalid    = "repo_path_invalid"
)

type RepoSetup struct {
	State    string
	Reason   string
	RepoHead string
}

func classifyRepoSetup(repoPath string) RepoSetup {
	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		return RepoSetup{State: StateSetupBlocked, Reason: SetupReasonRepoPathUnset}
	}

	info, err := os.Stat(repoPath)
	if err != nil {
		switch {
		case errors.Is(err, os.ErrNotExist):
			return RepoSetup{State: StateSetupBlocked, Reason: SetupReasonRepoPathMissing}
		case errors.Is(err, os.ErrPermission):
			return RepoSetup{State: StateSetupBlocked, Reason: SetupReasonRepoPathUnreadable}
		default:
			return RepoSetup{State: StateSetupBlocked, Reason: SetupReasonRepoPathInvalid}
		}
	}
	if !info.IsDir() {
		return RepoSetup{State: StateSetupBlocked, Reason: SetupReasonRepoPathInvalid}
	}

	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err != nil {
		switch {
		case errors.Is(err, os.ErrNotExist):
			return RepoSetup{State: StateSetupBlocked, Reason: SetupReasonRepoPathNotGit}
		case errors.Is(err, os.ErrPermission):
			return RepoSetup{State: StateSetupBlocked, Reason: SetupReasonRepoPathUnreadable}
		default:
			return RepoSetup{State: StateSetupBlocked, Reason: SetupReasonRepoPathInvalid}
		}
	}

	setup := RepoSetup{}
	if repoHead, ok := lookupRepoHead(repoPath); ok {
		setup.RepoHead = repoHead
	}
	return setup
}
