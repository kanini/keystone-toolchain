package toolchain

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kanini/keystone-toolchain/internal/contract"
	"github.com/kanini/keystone-toolchain/internal/runtime"
)

const (
	InitDirCreated          = "created"
	InitDirExisting         = "existing"
	InitDirWouldCreate      = "would_create"
	InitShellUpdated        = "updated"
	InitShellAlreadyPresent = "already_configured"
	InitShellUnsupported    = "unsupported"
	InitShellWouldUpdate    = "would_update"
)

const (
	shellBootstrapStart = "# >>> kstoolchain managed bin >>>"
	shellBootstrapEnd   = "# <<< kstoolchain managed bin <<<"
)

type InitBootstrap struct {
	ToolchainRoot       string    `json:"toolchain_root"`
	ManagedBinDir       string    `json:"managed_bin_dir"`
	StateDir            string    `json:"state_dir"`
	Shell               string    `json:"shell,omitempty"`
	RCPath              string    `json:"rc_path,omitempty"`
	RCStatus            string    `json:"rc_status,omitempty"`
	ManagedBinActive    bool      `json:"managed_bin_active"`
	ShellReloadRequired bool      `json:"shell_reload_required,omitempty"`
	ShellReloadCommand  string    `json:"shell_reload_command,omitempty"`
	Dirs                []InitDir `json:"dirs,omitempty"`
}

type InitDir struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

type shellTarget struct {
	name string
	path string
}

func applyInitBootstrap(report *InitReport, ctx *runtime.Context, opts InitOptions, overlayPath string) *contract.AppError {
	toolchainRoot := filepath.Join(ctx.HomeDir, ".keystone", "toolchain")
	report.Bootstrap = InitBootstrap{
		ToolchainRoot: toolchainRoot,
		ManagedBinDir: ctx.Config.ManagedBinDir,
		StateDir:      ctx.Config.StateDir,
	}

	for _, dirPath := range orderedUniquePaths(
		toolchainRoot,
		ctx.Config.ManagedBinDir,
		ctx.Config.StateDir,
		filepath.Dir(overlayPath),
	) {
		status, appErr := ensureInitDir(dirPath, opts.DryRun)
		if appErr != nil {
			return appErr
		}
		report.Bootstrap.Dirs = append(report.Bootstrap.Dirs, InitDir{Path: dirPath, Status: status})
		switch status {
		case InitDirCreated:
			report.ChangedItems = append(report.ChangedItems, fmt.Sprintf("created directory %s", dirPath))
		case InitDirWouldCreate:
			report.ChangedItems = append(report.ChangedItems, fmt.Sprintf("would create directory %s", dirPath))
		default:
			report.AlreadyCorrect = append(report.AlreadyCorrect, fmt.Sprintf("directory already exists: %s", dirPath))
		}
	}

	target, supported, appErr := resolveShellTarget(ctx.HomeDir, opts.Shell)
	if appErr != nil {
		return appErr
	}
	report.Bootstrap.Shell = target.name
	report.Bootstrap.RCPath = target.path
	report.Bootstrap.ManagedBinActive = managedBinFirstOnPATH(ctx.Config.ManagedBinDir)

	if !supported {
		report.Bootstrap.RCStatus = InitShellUnsupported
		report.ManualActions = appendUniqueStrings(report.ManualActions, fmt.Sprintf("prepend %s to PATH in your shell startup file, then run `kstoolchain init` again", ctx.Config.ManagedBinDir))
		report.delegateReadySet = false
		report.exitCode = contract.ExitValidation
		return nil
	}

	rcStatus, appErr := reconcileShellBootstrap(target, ctx.Config.ManagedBinDir, ctx.HomeDir, opts.DryRun)
	if appErr != nil {
		return appErr
	}
	report.Bootstrap.RCStatus = rcStatus
	switch rcStatus {
	case InitShellUpdated:
		report.ChangedItems = append(report.ChangedItems, fmt.Sprintf("updated %s to prepend %s on PATH", target.path, ctx.Config.ManagedBinDir))
	case InitShellWouldUpdate:
		report.ChangedItems = append(report.ChangedItems, fmt.Sprintf("would update %s to prepend %s on PATH", target.path, ctx.Config.ManagedBinDir))
	default:
		report.AlreadyCorrect = append(report.AlreadyCorrect, fmt.Sprintf("%s already prepends %s on PATH", target.path, ctx.Config.ManagedBinDir))
	}

	if !report.Bootstrap.ManagedBinActive {
		report.Bootstrap.ShellReloadRequired = true
		report.Bootstrap.ShellReloadCommand = shellReloadCommand(target)
		report.ManualActions = appendUniqueStrings(report.ManualActions, shellReloadAction(report.Bootstrap, opts.DryRun))
	}

	return nil
}

func orderedUniquePaths(paths ...string) []string {
	out := make([]string, 0, len(paths))
	seen := map[string]struct{}{}
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		path = filepath.Clean(path)
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
}

func ensureInitDir(path string, dryRun bool) (string, *contract.AppError) {
	if info, err := os.Stat(path); err == nil {
		if !info.IsDir() {
			return "", contract.Validation(contract.CodeConfigInvalid, fmt.Sprintf("Init path %s exists but is not a directory.", path), "Remove or rename the conflicting path and retry.", contract.Detail{Name: "path", Value: path})
		}
		return InitDirExisting, nil
	} else if !os.IsNotExist(err) {
		return "", contract.Infra(contract.CodeIOError, "Could not inspect init directory.", "Check filesystem permissions and retry.", err, contract.Detail{Name: "path", Value: path})
	}

	if dryRun {
		return InitDirWouldCreate, nil
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", contract.Infra(contract.CodeIOError, "Could not create init directory.", "Check filesystem permissions and retry.", err, contract.Detail{Name: "path", Value: path})
	}
	return InitDirCreated, nil
}

func resolveShellTarget(home, override string) (shellTarget, bool, *contract.AppError) {
	name := strings.TrimSpace(override)
	if name == "" {
		name = filepath.Base(strings.TrimSpace(os.Getenv("SHELL")))
	}
	if name == "" {
		name = "sh"
	}

	switch name {
	case "bash":
		return shellTarget{name: "bash", path: filepath.Join(home, ".bashrc")}, true, nil
	case "zsh":
		return shellTarget{name: "zsh", path: filepath.Join(home, ".zshrc")}, true, nil
	case "sh":
		return shellTarget{name: "sh", path: filepath.Join(home, ".profile")}, true, nil
	default:
		if strings.TrimSpace(override) != "" {
			return shellTarget{}, false, contract.ArgsInvalid("--shell must be one of bash, zsh, or sh.", "Use --shell bash|zsh|sh.")
		}
		return shellTarget{name: name}, false, nil
	}
}

func reconcileShellBootstrap(target shellTarget, managedBinDir, home string, dryRun bool) (string, *contract.AppError) {
	block := shellBootstrapBlock(target, managedBinDir, home)
	exportLine := shellExportLine(managedBinDir, home)

	content := ""
	info, err := os.Stat(target.path)
	mode := os.FileMode(0o644)
	if err == nil {
		mode = info.Mode()
		rawBytes, readErr := os.ReadFile(target.path)
		if readErr != nil {
			return "", contract.Infra(contract.CodeIOError, "Could not read shell rc file.", "Check file permissions and retry.", readErr, contract.Detail{Name: "path", Value: target.path})
		}
		content = string(rawBytes)
	} else if !os.IsNotExist(err) {
		return "", contract.Infra(contract.CodeIOError, "Could not inspect shell rc file.", "Check file permissions and retry.", err, contract.Detail{Name: "path", Value: target.path})
	}

	next := content
	switch {
	case strings.Contains(content, shellBootstrapStart) && strings.Contains(content, shellBootstrapEnd):
		next = replaceManagedBlock(content, block)
	case strings.Contains(content, exportLine):
		return InitShellAlreadyPresent, nil
	default:
		next = appendManagedBlock(content, block)
	}

	if next == content {
		return InitShellAlreadyPresent, nil
	}
	if dryRun {
		return InitShellWouldUpdate, nil
	}
	if appErr := writeTextFileAtomically(target.path, next, mode); appErr != nil {
		return "", appErr
	}
	return InitShellUpdated, nil
}

func shellBootstrapBlock(target shellTarget, managedBinDir, home string) string {
	return fmt.Sprintf("%s\n%s\n%s\n", shellBootstrapStart, shellExportLine(managedBinDir, home), shellBootstrapEnd)
}

func shellExportLine(managedBinDir, home string) string {
	return fmt.Sprintf("export PATH=\"%s:$PATH\"", shellPathLiteral(managedBinDir, home))
}

func shellPathLiteral(managedBinDir, home string) string {
	managedBinDir = filepath.Clean(managedBinDir)
	home = filepath.Clean(home)
	if managedBinDir == home {
		return "$HOME"
	}
	prefix := home + string(filepath.Separator)
	if strings.HasPrefix(managedBinDir, prefix) {
		return "$HOME/" + filepath.ToSlash(strings.TrimPrefix(managedBinDir, prefix))
	}
	return strings.ReplaceAll(managedBinDir, "\"", "\\\"")
}

func replaceManagedBlock(content, block string) string {
	start := strings.Index(content, shellBootstrapStart)
	end := strings.Index(content, shellBootstrapEnd)
	if start == -1 || end == -1 || end < start {
		return appendManagedBlock(content, block)
	}
	end += len(shellBootstrapEnd)
	if end < len(content) && content[end] == '\n' {
		end++
	}
	return content[:start] + block + content[end:]
}

func appendManagedBlock(content, block string) string {
	if strings.TrimSpace(content) == "" {
		return block
	}
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content + "\n" + block
}

func writeTextFileAtomically(path, body string, mode os.FileMode) *contract.AppError {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return contract.Infra(contract.CodeIOError, "Could not create shell rc directory.", "Check filesystem permissions and retry.", err, contract.Detail{Name: "path", Value: filepath.Dir(path)})
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return contract.Infra(contract.CodeIOError, "Could not create temp shell rc file.", "Check filesystem permissions and retry.", err, contract.Detail{Name: "path", Value: filepath.Dir(path)})
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.WriteString(body); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return contract.Infra(contract.CodeIOError, "Could not write temp shell rc file.", "Retry after checking filesystem health.", err, contract.Detail{Name: "path", Value: tmpPath})
	}
	if err := tmpFile.Chmod(mode); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return contract.Infra(contract.CodeIOError, "Could not set shell rc permissions.", "Retry after checking filesystem health.", err, contract.Detail{Name: "path", Value: tmpPath})
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return contract.Infra(contract.CodeIOError, "Could not close temp shell rc file.", "Retry after checking filesystem health.", err, contract.Detail{Name: "path", Value: tmpPath})
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return contract.Infra(contract.CodeIOError, "Could not atomically write shell rc file.", "Check that the shell rc path is writable and retry.", err, contract.Detail{Name: "path", Value: path})
	}
	return nil
}

func managedBinFirstOnPATH(managedBinDir string) bool {
	managedBinDir = filepath.Clean(strings.TrimSpace(managedBinDir))
	if managedBinDir == "" {
		return false
	}
	pathEnv := strings.TrimSpace(os.Getenv("PATH"))
	if pathEnv == "" {
		return false
	}
	parts := strings.Split(pathEnv, string(os.PathListSeparator))
	for _, part := range parts {
		part = filepath.Clean(strings.TrimSpace(part))
		if part == "" {
			continue
		}
		return part == managedBinDir
	}
	return false
}

func shellReloadCommand(target shellTarget) string {
	switch target.name {
	case "sh":
		return ". " + target.path
	default:
		return "source " + target.path
	}
}

func shellReloadAction(bootstrap InitBootstrap, dryRun bool) string {
	if bootstrap.ShellReloadCommand == "" {
		return ""
	}
	if dryRun {
		return fmt.Sprintf("after a real init run, open a new shell or run `%s` so %s wins on PATH", bootstrap.ShellReloadCommand, bootstrap.ManagedBinDir)
	}
	return fmt.Sprintf("open a new shell or run `%s` so %s wins on PATH", bootstrap.ShellReloadCommand, bootstrap.ManagedBinDir)
}
