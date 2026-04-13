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
	persisted, stateFile, statePresent, appErr := toolchain.LoadPersistedState(s.ctx)
	if appErr != nil {
		return toolchain.StatusReport{}, appErr.Exit, appErr
	}
	report := toolchain.BuildStatusReport(s.ctx, manifest, persisted, stateFile, statePresent)
	return report, toolchain.StatusExitCode(report), nil
}
