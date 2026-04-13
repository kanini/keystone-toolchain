package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kanini/keystone-toolchain/internal/contract"
	"github.com/kanini/keystone-toolchain/internal/runtime"
	"github.com/kanini/keystone-toolchain/internal/service"
	"github.com/kanini/keystone-toolchain/internal/toolchain"
)

type app struct {
	root   *cobra.Command
	opts   runtime.GlobalOptions
	stdout io.Writer
	stderr io.Writer
}

type exitError struct {
	code int
}

func (e exitError) Error() string {
	return fmt.Sprintf("exit %d", e.code)
}

func Execute() int {
	a := newApp(os.Stdout, os.Stderr)
	return executeApp(a)
}

func executeApp(a *app) int {
	if err := a.root.Execute(); err != nil {
		if ex, ok := err.(exitError); ok {
			return ex.code
		}
		appErr := mapCobraError(err)
		a.renderStandaloneFailure(appErr)
		return appErr.Exit
	}
	return contract.ExitOK
}

func newApp(stdout, stderr io.Writer) *app {
	a := &app{stdout: stdout, stderr: stderr}
	root := &cobra.Command{
		Use:   "kstoolchain",
		Short: "Keystone toolchain sync and status CLI",
		Long: `kstoolchain keeps Keystone tools current and truthful.

The first release surface is small on purpose:
- version reports build provenance
- status will report managed tool truth
- sync will refresh ready adapters into the Keystone bin dir

The scaffold already carries the load-bearing CLI pieces:
- build provenance
- contract-first output
- config and runtime context
- install and staleness checks`,
		Example: `  kstoolchain version
  kstoolchain version --json
  kstoolchain status
  kstoolchain sync`,
		Version:       contract.VersionString(),
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			name := cmd.Name()
			if name == "__complete" || name == "completion" {
				return
			}
			if a.opts.JSON || strings.EqualFold(strings.TrimSpace(a.opts.Format), "json") {
				return
			}
			if warning := contract.CheckStaleness("kstoolchain"); warning != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "WARN %s: %s\nHint: %s\n", warning.Code, warning.Message, warning.Hint)
			}
		},
	}
	root.SetVersionTemplate("{{.Version}}\n")
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return err
	})

	root.PersistentFlags().StringVar(&a.opts.ConfigPath, "config", "", "Config file path")
	root.PersistentFlags().StringVar(&a.opts.Format, "format", "text", "Output format: text|json")
	root.PersistentFlags().BoolVar(&a.opts.JSON, "json", false, "Alias for --format json")
	root.PersistentFlags().StringVar(&a.opts.ManagedBinDir, "managed-bin-dir", "", "Override managed bin dir")
	root.PersistentFlags().StringVar(&a.opts.StateDir, "state-dir", "", "Override state dir")

	root.AddCommand(a.newVersionCmd())
	root.AddCommand(a.newSyncCmd())
	root.AddCommand(a.newStatusCmd())

	a.root = root
	return a
}

func (a *app) runCommand(handler func(*runtime.Context, *service.Service) (any, []string, []contract.Warning, int, *contract.AppError)) error {
	ctx, appErr := runtime.BuildContext(a.opts)
	if appErr != nil {
		a.renderStandaloneFailure(appErr)
		return exitError{code: appErr.Exit}
	}

	svc := service.New(ctx)
	result, textLines, warnings, exitCode, cmdErr := handler(ctx, svc)
	if cmdErr != nil {
		a.renderFailure(ctx, warnings, cmdErr)
		return exitError{code: cmdErr.Exit}
	}

	a.renderSuccess(ctx, result, textLines, warnings)
	if exitCode != contract.ExitOK {
		return exitError{code: exitCode}
	}
	return nil
}

func (a *app) newVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version and build provenance",
		Long: `Print kstoolchain version and build provenance.

Text mode is for people. JSON mode is for tools and agents.`,
		Example: `  kstoolchain version
  kstoolchain version --json`,
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) != 0 {
				return a.runCommand(func(_ *runtime.Context, _ *service.Service) (any, []string, []contract.Warning, int, *contract.AppError) {
					return nil, nil, nil, contract.ExitValidation, contract.ArgsInvalid("version takes no arguments.", "Run: kstoolchain version")
				})
			}
			return a.runCommand(func(_ *runtime.Context, svc *service.Service) (any, []string, []contract.Warning, int, *contract.AppError) {
				info := svc.Version()
				return info, []string{contract.VersionString()}, nil, contract.ExitOK, nil
			})
		},
	}
	return cmd
}

func (a *app) newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync managed Keystone tools",
		Long: `Sync managed Keystone tools into the configured Keystone bin dir.

Sync is intentionally narrow in v1:
- it operates on ready adapters only
- it stages, probes, and then promotes
- it writes current.json so status can read the result back truthfully`,
		Example: `  kstoolchain sync
  kstoolchain sync --json`,
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) != 0 {
				return a.runCommand(func(_ *runtime.Context, _ *service.Service) (any, []string, []contract.Warning, int, *contract.AppError) {
					return nil, nil, nil, contract.ExitValidation, contract.ArgsInvalid("sync takes no arguments.", "Run: kstoolchain sync")
				})
			}
			return a.runCommand(func(_ *runtime.Context, svc *service.Service) (any, []string, []contract.Warning, int, *contract.AppError) {
				report, exitCode, appErr := svc.SyncReport()
				if appErr != nil {
					return nil, nil, nil, exitCode, appErr
				}
				return report, toolchain.RenderStatusText(report), nil, exitCode, nil
			})
		},
	}
	return cmd
}

func (a *app) newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show managed tool status",
		Long: `Show managed Keystone tool status.

Status is the live truth surface:
- tracked inventory comes from the manifest
- persisted tool state comes from current.json
- PATH resolution decides what the shell will really run`,
		Example: `  kstoolchain status
  kstoolchain status --json`,
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) != 0 {
				return a.runCommand(func(_ *runtime.Context, _ *service.Service) (any, []string, []contract.Warning, int, *contract.AppError) {
					return nil, nil, nil, contract.ExitValidation, contract.ArgsInvalid("status takes no arguments.", "Run: kstoolchain status")
				})
			}
			return a.runCommand(func(_ *runtime.Context, svc *service.Service) (any, []string, []contract.Warning, int, *contract.AppError) {
				report, exitCode, appErr := svc.StatusReport()
				if appErr != nil {
					return nil, nil, nil, exitCode, appErr
				}
				return report, toolchain.RenderStatusText(report), nil, exitCode, nil
			})
		},
	}
	return cmd
}

func (a *app) renderSuccess(ctx *runtime.Context, result any, textLines []string, warnings []contract.Warning) {
	if ctx.IsJSON {
		a.writeJSON(contract.Success(result, warnings))
		return
	}
	for _, warning := range warnings {
		fmt.Fprintf(a.stderr, "WARN %s: %s\n", warning.Code, warning.Message)
		if warning.Hint != "" {
			fmt.Fprintf(a.stderr, "Hint: %s\n", warning.Hint)
		}
	}
	for _, line := range textLines {
		fmt.Fprintln(a.stdout, line)
	}
}

func (a *app) renderFailure(ctx *runtime.Context, warnings []contract.Warning, appErr *contract.AppError) {
	if ctx != nil && ctx.IsJSON {
		a.writeJSON(contract.Failure(appErr, warnings))
		return
	}
	a.renderTextFailure(warnings, appErr)
}

func (a *app) renderStandaloneFailure(appErr *contract.AppError) {
	if wantsJSON(a.opts) {
		a.writeJSON(contract.Failure(appErr, nil))
		return
	}
	a.renderTextFailure(nil, appErr)
}

func (a *app) renderTextFailure(warnings []contract.Warning, appErr *contract.AppError) {
	for _, warning := range warnings {
		fmt.Fprintf(a.stderr, "WARN %s: %s\n", warning.Code, warning.Message)
		if warning.Hint != "" {
			fmt.Fprintf(a.stderr, "Hint: %s\n", warning.Hint)
		}
	}
	fmt.Fprintf(a.stdout, "FAIL %s %s\n", appErr.Code, appErr.Message)
	if appErr.Hint != "" {
		fmt.Fprintf(a.stdout, "Hint: %s\n", appErr.Hint)
	}
}

func (a *app) writeJSON(payload contract.Envelope) {
	enc := json.NewEncoder(a.stdout)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(payload)
}

func mapCobraError(err error) *contract.AppError {
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		msg = "Unknown command error."
	}
	return contract.ArgsInvalid(msg, "Run: kstoolchain --help")
}

func wantsJSON(opts runtime.GlobalOptions) bool {
	if opts.JSON {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(opts.Format), "json")
}
