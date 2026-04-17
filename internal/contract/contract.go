package contract

import (
	"errors"
	"fmt"
	"runtime/debug"
)

const ContractVersion = 1

var (
	KSToolchainVersion = "0.1.0"
	BuildCommit        = "unknown"
	BuildDate          = "unknown"
	BuildSource        = "dev"
	SourceRepo         = ""
)

const (
	CodeArgsInvalid     = "ARGS_INVALID"
	CodeConfigInvalid   = "CONFIG_INVALID"
	CodeIOError         = "IO_ERROR"
	CodeNotImplemented  = "NOT_IMPLEMENTED"
	CodeOverlayMissing  = "OVERLAY_MISSING"
	CodeOverlayUnknown  = "OVERLAY_UNKNOWN_REPO"
	CodeOverlayInvalid  = "OVERLAY_INVALID"
	CodeOverlayIO       = "OVERLAY_UNREADABLE"
	CodeOverlayDupID    = "OVERLAY_DUPLICATE_REPO"
	KindValidation      = "validation"
	KindInfrastructure  = "infrastructure"
	WarningStaleBinary  = "STALE_BINARY"
	ExitOK              = 0
	ExitValidation      = 1
	ExitReadySetBlocked = 2
	ExitInfrastructure  = 12
)

type Detail struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

type AppError struct {
	Kind    string
	Code    string
	Message string
	Hint    string
	Details []Detail
	Exit    int
	Err     error
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func Validation(code, msg, hint string, details ...Detail) *AppError {
	return &AppError{
		Kind:    KindValidation,
		Code:    code,
		Message: msg,
		Hint:    hint,
		Details: details,
		Exit:    ExitValidation,
	}
}

func Infra(code, msg, hint string, cause error, details ...Detail) *AppError {
	return &AppError{
		Kind:    KindInfrastructure,
		Code:    code,
		Message: msg,
		Hint:    hint,
		Details: details,
		Exit:    ExitInfrastructure,
		Err:     cause,
	}
}

func ArgsInvalid(msg, hint string, details ...Detail) *AppError {
	return Validation(CodeArgsInvalid, msg, hint, details...)
}

func NotImplemented(msg, hint string, details ...Detail) *AppError {
	return Validation(CodeNotImplemented, msg, hint, details...)
}

func AsAppError(err error) *AppError {
	if err == nil {
		return nil
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return Infra(CodeIOError, "Unexpected runtime error.", "Retry the command.", err)
}

type VersionInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
	Source  string `json:"source"`
	Repo    string `json:"repo,omitempty"`
	Dirty   bool   `json:"dirty,omitempty"`
}

func CurrentVersionInfo() VersionInfo {
	info := VersionInfo{
		Name:    "kstoolchain",
		Version: KSToolchainVersion,
		Commit:  BuildCommit,
		Date:    BuildDate,
		Source:  BuildSource,
		Repo:    SourceRepo,
	}

	if info.Commit != "unknown" && info.Date != "unknown" {
		return info
	}

	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return info
	}
	for _, setting := range buildInfo.Settings {
		switch setting.Key {
		case "vcs.revision":
			if info.Commit == "unknown" {
				info.Commit = setting.Value
			}
		case "vcs.time":
			if info.Date == "unknown" {
				info.Date = setting.Value
			}
		case "vcs.modified":
			if setting.Value == "true" {
				info.Dirty = true
			}
		}
	}
	return info
}

func VersionString() string {
	info := CurrentVersionInfo()
	commit := short(info.Commit)
	dirty := ""
	if info.Dirty {
		dirty = "-dirty"
	}
	return fmt.Sprintf(
		"%s version %s commit=%s%s date=%s source=%s",
		info.Name,
		info.Version,
		commit,
		dirty,
		info.Date,
		info.Source,
	)
}

type Envelope struct {
	ContractVersion int            `json:"contract_version"`
	OK              bool           `json:"ok"`
	Result          any            `json:"result,omitempty"`
	Error           *ErrorEnvelope `json:"error,omitempty"`
	Warnings        []Warning      `json:"warnings,omitempty"`
}

type ErrorEnvelope struct {
	Kind    string   `json:"kind"`
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Hint    string   `json:"hint,omitempty"`
	Details []Detail `json:"details,omitempty"`
}

func Success(result any, warnings []Warning) Envelope {
	return Envelope{
		ContractVersion: ContractVersion,
		OK:              true,
		Result:          result,
		Warnings:        warnings,
	}
}

func NonSuccess(result any, warnings []Warning) Envelope {
	return Envelope{
		ContractVersion: ContractVersion,
		OK:              false,
		Result:          result,
		Warnings:        warnings,
	}
}

func Failure(appErr *AppError, warnings []Warning) Envelope {
	if appErr == nil {
		appErr = Infra(CodeIOError, "Unknown error.", "Retry the command.", nil)
	}
	return Envelope{
		ContractVersion: ContractVersion,
		OK:              false,
		Error: &ErrorEnvelope{
			Kind:    appErr.Kind,
			Code:    appErr.Code,
			Message: appErr.Message,
			Hint:    appErr.Hint,
			Details: appErr.Details,
		},
		Warnings: warnings,
	}
}

func short(v string) string {
	if len(v) > 7 {
		return v[:7]
	}
	if v == "" {
		return "unknown"
	}
	return v
}
