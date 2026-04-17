package toolchain

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/kanini/keystone-toolchain/internal/contract"
	"github.com/kanini/keystone-toolchain/internal/runtime"
)

const (
	SyncAttemptSchema = "kstoolchain.sync-attempt/v1alpha1"

	syncLockFileName    = "sync.lock"
	syncAttemptFileName = "attempt.json"

	AttemptPhasePrePromotion      = "pre_promotion"
	AttemptPhasePromotionOrLater  = "promotion_or_later"
	AttemptIntegrityPrePromotion  = "UNRESOLVED_PRE_PROMOTION"
	AttemptIntegrityPromotionLate = "UNRESOLVED_PROMOTION_OR_LATER"
	AttemptIntegrityArtifactBad   = "ATTEMPT_ARTIFACT_INVALID"
)

type SyncAttemptArtifact struct {
	Schema                 string   `json:"schema"`
	AttemptID              string   `json:"attempt_id"`
	StartedAt              string   `json:"started_at"`
	OwnerHost              string   `json:"owner_host,omitempty"`
	OwnerPID               int      `json:"owner_pid,omitempty"`
	ReadyRepoIDs           []string `json:"ready_repo_ids"`
	Phase                  string   `json:"phase"`
	CarriedUnresolvedPhase string   `json:"carried_unresolved_phase,omitempty"`
}

type AttemptIntegrityStatus struct {
	State                  string `json:"state"`
	Reason                 string `json:"reason,omitempty"`
	AttemptID              string `json:"attempt_id,omitempty"`
	StartedAt              string `json:"started_at,omitempty"`
	OwnerHost              string `json:"owner_host,omitempty"`
	OwnerPID               int    `json:"owner_pid,omitempty"`
	Phase                  string `json:"phase,omitempty"`
	CarriedUnresolvedPhase string `json:"carried_unresolved_phase,omitempty"`
}

type SyncLock struct {
	file *os.File
}

type SyncAttemptController struct {
	ctx      *runtime.Context
	artifact SyncAttemptArtifact
}

type attemptArtifactLoad struct {
	artifact SyncAttemptArtifact
	present  bool
	invalid  bool
	reason   string
	cause    error
}

func AcquireSyncLock(ctx *runtime.Context) (*SyncLock, *contract.AppError) {
	if err := os.MkdirAll(ctx.Config.StateDir, 0o755); err != nil {
		return nil, contract.Infra(contract.CodeIOError, "Could not create toolchain state dir.", "Check permissions for the state dir and retry.", err, contract.Detail{Name: "path", Value: ctx.Config.StateDir})
	}

	lockPath := filepath.Join(ctx.Config.StateDir, syncLockFileName)
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, contract.Infra(contract.CodeIOError, "Could not open sync lock file.", "Check permissions for the state dir and retry.", err, contract.Detail{Name: "path", Value: lockPath})
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, contract.Validation(contract.CodeSyncBusy, "Another kstoolchain sync is already running.", "Wait for the active sync to finish, then rerun `kstoolchain sync`.", contract.Detail{Name: "path", Value: lockPath})
		}
		return nil, contract.Infra(contract.CodeIOError, "Could not acquire sync lock.", "Retry after checking filesystem health.", err, contract.Detail{Name: "path", Value: lockPath})
	}
	return &SyncLock{file: file}, nil
}

func (lock *SyncLock) Close() error {
	if lock == nil || lock.file == nil {
		return nil
	}
	err := syscall.Flock(int(lock.file.Fd()), syscall.LOCK_UN)
	closeErr := lock.file.Close()
	lock.file = nil
	if err != nil {
		return err
	}
	return closeErr
}

func BeginSyncAttempt(ctx *runtime.Context, ready []RepoAdapter, committedAttemptID string) (*SyncAttemptController, *contract.AppError) {
	load := loadAttemptArtifact(ctx)
	if appErr := syncAttemptLoadError(ctx, load); appErr != nil {
		return nil, appErr
	}

	attemptID, err := generateAttemptID()
	if err != nil {
		return nil, contract.Infra(contract.CodeIOError, "Could not generate sync attempt id.", "Retry the command.", err)
	}

	artifact := SyncAttemptArtifact{
		Schema:       SyncAttemptSchema,
		AttemptID:    attemptID,
		StartedAt:    ctx.Now().UTC().Format("2006-01-02T15:04:05Z"),
		OwnerPID:     os.Getpid(),
		ReadyRepoIDs: readyRepoIDs(ready),
		Phase:        AttemptPhasePrePromotion,
	}
	if host, err := os.Hostname(); err == nil {
		artifact.OwnerHost = strings.TrimSpace(host)
	}
	if load.present && strings.TrimSpace(load.artifact.AttemptID) != strings.TrimSpace(committedAttemptID) {
		artifact.CarriedUnresolvedPhase = strictestAttemptPhase(load.artifact.Phase, load.artifact.CarriedUnresolvedPhase)
	}

	controller := &SyncAttemptController{ctx: ctx, artifact: artifact}
	if appErr := controller.persist(); appErr != nil {
		return nil, appErr
	}
	return controller, nil
}

func (controller *SyncAttemptController) AttemptID() string {
	if controller == nil {
		return ""
	}
	return controller.artifact.AttemptID
}

func (controller *SyncAttemptController) EnsurePromotionOrLater() *contract.AppError {
	if controller == nil || controller.artifact.Phase == AttemptPhasePromotionOrLater {
		return nil
	}
	controller.artifact.Phase = AttemptPhasePromotionOrLater
	return controller.persist()
}

func (controller *SyncAttemptController) ClearCarriedAfterCommit() *contract.AppError {
	if controller == nil || controller.artifact.CarriedUnresolvedPhase == "" {
		return nil
	}
	controller.artifact.CarriedUnresolvedPhase = ""
	return controller.persist()
}

func (controller *SyncAttemptController) persist() *contract.AppError {
	return saveAttemptArtifact(controller.ctx, controller.artifact)
}

func syncAttemptPath(ctx *runtime.Context) string {
	return filepath.Join(ctx.Config.StateDir, syncAttemptFileName)
}

func saveAttemptArtifact(ctx *runtime.Context, artifact SyncAttemptArtifact) *contract.AppError {
	if err := os.MkdirAll(ctx.Config.StateDir, 0o755); err != nil {
		return contract.Infra(contract.CodeIOError, "Could not create toolchain state dir.", "Check permissions for the state dir and retry.", err, contract.Detail{Name: "path", Value: ctx.Config.StateDir})
	}
	artifact.Schema = SyncAttemptSchema
	if err := validateSyncAttemptArtifact(artifact); err != nil {
		return contract.Validation(contract.CodeConfigInvalid, "Sync attempt artifact is invalid.", "Fix or remove attempt.json, then rerun `kstoolchain sync`.", contract.Detail{Name: "path", Value: syncAttemptPath(ctx)}, contract.Detail{Name: "reason", Value: err.Error()})
	}
	if err := writeJSONAtomically(syncAttemptPath(ctx), artifact, true); err != nil {
		return contract.Infra(contract.CodeIOError, "Could not atomically write sync attempt artifact.", "Check that the state dir is writable and retry.", err, contract.Detail{Name: "path", Value: syncAttemptPath(ctx)})
	}
	return nil
}

func loadAttemptArtifact(ctx *runtime.Context) attemptArtifactLoad {
	path := syncAttemptPath(ctx)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return attemptArtifactLoad{}
		}
		return attemptArtifactLoad{
			present: true,
			invalid: true,
			reason:  truncateReason(fmt.Sprintf("attempt artifact could not be read: %v", err)),
			cause:   err,
		}
	}

	var artifact SyncAttemptArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return attemptArtifactLoad{
			present: true,
			invalid: true,
			reason:  "attempt artifact is not valid JSON",
		}
	}
	if err := validateSyncAttemptArtifact(artifact); err != nil {
		return attemptArtifactLoad{
			present: true,
			invalid: true,
			reason:  truncateReason(err.Error()),
		}
	}
	return attemptArtifactLoad{artifact: artifact, present: true}
}

func syncAttemptLoadError(ctx *runtime.Context, load attemptArtifactLoad) *contract.AppError {
	if !load.invalid {
		return nil
	}
	path := syncAttemptPath(ctx)
	if load.cause != nil {
		return contract.Infra(contract.CodeIOError, "Could not read sync attempt artifact.", "Fix file permissions or remove attempt.json, then rerun `kstoolchain sync`.", load.cause, contract.Detail{Name: "path", Value: path})
	}
	return contract.Validation(contract.CodeConfigInvalid, "Sync attempt artifact is invalid.", "Fix or remove attempt.json, then rerun `kstoolchain sync`.", contract.Detail{Name: "path", Value: path}, contract.Detail{Name: "reason", Value: load.reason})
}

func ProjectAttemptIntegrity(ctx *runtime.Context, committedAttemptID string) *AttemptIntegrityStatus {
	load := loadAttemptArtifact(ctx)
	if !load.present && !load.invalid {
		return nil
	}
	if load.invalid {
		return &AttemptIntegrityStatus{
			State:  AttemptIntegrityArtifactBad,
			Reason: load.reason,
		}
	}
	if strings.TrimSpace(committedAttemptID) != "" && strings.TrimSpace(committedAttemptID) == strings.TrimSpace(load.artifact.AttemptID) {
		return nil
	}

	effectivePhase := strictestAttemptPhase(load.artifact.Phase, load.artifact.CarriedUnresolvedPhase)
	state := AttemptIntegrityPrePromotion
	reason := "latest sync attempt has no correlated committed current.json yet"
	if effectivePhase == AttemptPhasePromotionOrLater {
		state = AttemptIntegrityPromotionLate
		reason = "latest sync attempt crossed the promotion boundary without a correlated committed current.json"
	}
	return &AttemptIntegrityStatus{
		State:                  state,
		Reason:                 reason,
		AttemptID:              load.artifact.AttemptID,
		StartedAt:              load.artifact.StartedAt,
		OwnerHost:              load.artifact.OwnerHost,
		OwnerPID:               load.artifact.OwnerPID,
		Phase:                  load.artifact.Phase,
		CarriedUnresolvedPhase: load.artifact.CarriedUnresolvedPhase,
	}
}

func validateSyncAttemptArtifact(artifact SyncAttemptArtifact) error {
	if strings.TrimSpace(artifact.Schema) != SyncAttemptSchema {
		return fmt.Errorf("attempt artifact schema is invalid")
	}
	if strings.TrimSpace(artifact.AttemptID) == "" {
		return fmt.Errorf("attempt artifact attempt_id is required")
	}
	if strings.TrimSpace(artifact.StartedAt) == "" {
		return fmt.Errorf("attempt artifact started_at is required")
	}
	if normalizeAttemptPhase(artifact.Phase) == "" {
		return fmt.Errorf("attempt artifact phase is invalid")
	}
	if artifact.CarriedUnresolvedPhase != "" && normalizeAttemptPhase(artifact.CarriedUnresolvedPhase) == "" {
		return fmt.Errorf("attempt artifact carried_unresolved_phase is invalid")
	}
	if len(artifact.ReadyRepoIDs) == 0 {
		return fmt.Errorf("attempt artifact ready_repo_ids is required")
	}
	seen := map[string]struct{}{}
	for _, repoID := range artifact.ReadyRepoIDs {
		repoID = strings.TrimSpace(repoID)
		if repoID == "" {
			return fmt.Errorf("attempt artifact ready_repo_ids entries must not be empty")
		}
		if _, ok := seen[repoID]; ok {
			return fmt.Errorf("attempt artifact duplicates ready repo_id %s", repoID)
		}
		seen[repoID] = struct{}{}
	}
	return nil
}

func normalizeAttemptPhase(raw string) string {
	switch strings.TrimSpace(raw) {
	case AttemptPhasePrePromotion:
		return AttemptPhasePrePromotion
	case AttemptPhasePromotionOrLater:
		return AttemptPhasePromotionOrLater
	default:
		return ""
	}
}

func strictestAttemptPhase(phases ...string) string {
	out := ""
	for _, phase := range phases {
		switch normalizeAttemptPhase(phase) {
		case AttemptPhasePromotionOrLater:
			return AttemptPhasePromotionOrLater
		case AttemptPhasePrePromotion:
			if out == "" {
				out = AttemptPhasePrePromotion
			}
		}
	}
	return out
}

func readyRepoIDs(ready []RepoAdapter) []string {
	ids := make([]string, 0, len(ready))
	for _, adapter := range ready {
		ids = append(ids, adapter.RepoID)
	}
	return ids
}

func generateAttemptID() (string, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("attempt-%x", random[:]), nil
}

func writeJSONAtomically(targetPath string, payload any, durable bool) error {
	dir := filepath.Dir(targetPath)
	tmpFile, err := os.CreateTemp(dir, filepath.Base(targetPath)+".tmp-*")
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

	enc := json.NewEncoder(tmpFile)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if durable {
		if err := tmpFile.Sync(); err != nil {
			_ = tmpFile.Close()
			return err
		}
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		return err
	}
	if durable {
		if err := syncDirectory(dir); err != nil {
			return err
		}
	}
	cleanup = false
	return nil
}

func syncDirectory(dirPath string) error {
	dir, err := os.Open(dirPath)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
