package service

import (
	"github.com/kanini/keystone-toolchain/internal/contract"
	"github.com/kanini/keystone-toolchain/internal/runtime"
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
