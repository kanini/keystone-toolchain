package service

import (
	"github.com/kanini/keystone-toolchain/internal/contract"
	"github.com/kanini/keystone-toolchain/internal/runtime"
	"github.com/kanini/keystone-toolchain/internal/toolchain"
)

type Service struct {
	ctx *runtime.Context
}

func New(ctx *runtime.Context) *Service {
	return &Service{ctx: ctx}
}

func (s *Service) Version() contract.VersionInfo {
	return contract.CurrentVersionInfo()
}

func (s *Service) Context() *runtime.Context {
	return s.ctx
}

func (s *Service) StatusReport() (toolchain.StatusReport, int, *contract.AppError) {
	manifest, appErr := toolchain.LoadManifest(s.ctx)
	if appErr != nil {
		return toolchain.StatusReport{}, contract.ExitValidation, appErr
	}
	persisted, stateFile, statePresent, contractDrift, appErr := toolchain.LoadPersistedState(s.ctx)
	if appErr != nil {
		return toolchain.StatusReport{}, appErr.Exit, appErr
	}
	report := toolchain.BuildStatusReport(s.ctx, manifest, persisted, stateFile, statePresent, contractDrift)
	return report, toolchain.StatusExitCode(report), nil
}

func (s *Service) SyncReport() (toolchain.StatusReport, int, *contract.AppError) {
	manifest, appErr := toolchain.LoadManifest(s.ctx)
	if appErr != nil {
		return toolchain.StatusReport{}, contract.ExitValidation, appErr
	}
	persisted, _, statePresent, _, appErr := toolchain.LoadPersistedState(s.ctx)
	if appErr != nil {
		return toolchain.StatusReport{}, appErr.Exit, appErr
	}
	if appErr := toolchain.SyncReadySet(s.ctx, manifest, persisted, statePresent); appErr != nil {
		return toolchain.StatusReport{}, appErr.Exit, appErr
	}
	readyManifest := manifest
	readyManifest.Repos = toolchain.SelectReadyAdapters(manifest)
	persisted, stateFile, statePresent, contractDrift, appErr := toolchain.LoadPersistedState(s.ctx)
	if appErr != nil {
		return toolchain.StatusReport{}, appErr.Exit, appErr
	}
	report := toolchain.BuildStatusReport(s.ctx, readyManifest, persisted, stateFile, statePresent, contractDrift)
	return report, toolchain.SyncExitCode(readyManifest, persisted), nil
}
