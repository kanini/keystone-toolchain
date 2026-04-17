package service

import (
	"io"

	"github.com/kanini/keystone-toolchain/internal/contract"
	"github.com/kanini/keystone-toolchain/internal/runtime"
	"github.com/kanini/keystone-toolchain/internal/toolchain"
)

type Service struct {
	ctx              *runtime.Context
	readySetExecutor func(toolchain.SyncOptions) (toolchain.StatusReport, []contract.Warning, int, *contract.AppError)
}

func New(ctx *runtime.Context) *Service {
	s := &Service{ctx: ctx}
	s.readySetExecutor = s.executeReadySetSync
	return s
}

func (s *Service) Version() contract.VersionInfo {
	return contract.CurrentVersionInfo()
}

func (s *Service) Context() *runtime.Context {
	return s.ctx
}

func (s *Service) StatusReport() (toolchain.StatusReport, []contract.Warning, int, *contract.AppError) {
	manifest, appErr := toolchain.LoadManifest(s.ctx)
	if appErr != nil {
		return toolchain.StatusReport{}, nil, contract.ExitValidation, appErr
	}
	warnings := toolchain.StatusOverlayWarnings(manifest)
	persisted, stateFile, statePresent, contractDrift, appErr := toolchain.LoadPersistedState(s.ctx)
	if appErr != nil {
		return toolchain.StatusReport{}, warnings, appErr.Exit, appErr
	}
	report := toolchain.BuildStatusReport(s.ctx, manifest, persisted, stateFile, statePresent, contractDrift)
	return report, warnings, toolchain.StatusExitCode(report), nil
}

func (s *Service) SyncReport(opts toolchain.SyncOptions) (toolchain.StatusReport, []contract.Warning, int, *contract.AppError) {
	return s.readySetExecutor(opts)
}

func (s *Service) InitReport(in io.Reader, out io.Writer, opts toolchain.InitOptions) (toolchain.InitReport, []contract.Warning, int, *contract.AppError) {
	report, appErr := toolchain.RunInitFlow(s.ctx, in, out, opts)
	if appErr != nil {
		return toolchain.InitReport{}, nil, appErr.Exit, appErr
	}
	if !report.ShouldDelegateReadySet() {
		return report, nil, report.ExitCode(), nil
	}

	readyReport, warnings, _, appErr := s.readySetExecutor(toolchain.SyncOptions{})
	if appErr != nil {
		return toolchain.InitReport{}, warnings, appErr.Exit, appErr
	}
	report.Delegated = true
	report.ReadySet = &readyReport
	report.ManualActions = append(toolchain.CollectReadySetManualActions(readyReport), report.ManualActions...)
	report.ManualActions = dedupeStrings(report.ManualActions)
	return report, warnings, toolchain.StatusExitCode(readyReport), nil
}

func (s *Service) executeReadySetSync(opts toolchain.SyncOptions) (toolchain.StatusReport, []contract.Warning, int, *contract.AppError) {
	manifest, appErr := toolchain.LoadManifest(s.ctx)
	if appErr != nil {
		return toolchain.StatusReport{}, nil, contract.ExitValidation, appErr
	}
	warnings := toolchain.SyncOverlayWarnings(manifest)
	if appErr := toolchain.SyncOverlayError(manifest); appErr != nil {
		return toolchain.StatusReport{}, warnings, appErr.Exit, appErr
	}
	persisted, _, statePresent, _, appErr := toolchain.LoadPersistedState(s.ctx)
	if appErr != nil {
		return toolchain.StatusReport{}, warnings, appErr.Exit, appErr
	}
	if appErr := toolchain.SyncReadySet(s.ctx, manifest, persisted, statePresent, opts); appErr != nil {
		return toolchain.StatusReport{}, warnings, appErr.Exit, appErr
	}
	readyManifest := manifest
	readyManifest.Repos = toolchain.SelectReadyAdapters(manifest)
	persisted, stateFile, statePresent, contractDrift, appErr := toolchain.LoadPersistedState(s.ctx)
	if appErr != nil {
		return toolchain.StatusReport{}, warnings, appErr.Exit, appErr
	}
	report := toolchain.BuildStatusReport(s.ctx, readyManifest, persisted, stateFile, statePresent, contractDrift)
	return report, warnings, toolchain.SyncExitCode(readyManifest, persisted), nil
}

func dedupeStrings(items []string) []string {
	out := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
