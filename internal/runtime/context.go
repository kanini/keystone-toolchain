package runtime

import (
	"os"
	"strings"
	"time"

	"github.com/kanini/keystone-toolchain/internal/contract"
)

type Context struct {
	HomeDir      string
	ConfigPath   string
	Config       Config
	Format       string
	IsJSON       bool
	AdaptersPath string
	Now          func() time.Time
}

func BuildContext(opts GlobalOptions) (*Context, *contract.AppError) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, contract.Infra(contract.CodeIOError, "Could not resolve home directory.", "Check HOME and retry.", err)
	}

	cfg, cfgPath, cfgErr := LoadConfig(home, opts)
	if cfgErr != nil {
		return nil, cfgErr
	}

	format := strings.TrimSpace(opts.Format)
	if format == "" {
		format = "text"
	}
	if opts.JSON {
		format = "json"
	}
	if format != "text" && format != "json" {
		return nil, contract.ArgsInvalid("--format must be text or json.", "Use --format text|json or --json.")
	}

	return &Context{
		HomeDir:      home,
		ConfigPath:   cfgPath,
		Config:       cfg,
		Format:       format,
		IsJSON:       format == "json",
		AdaptersPath: strings.TrimSpace(opts.AdaptersPath),
		Now:          time.Now,
	}, nil
}
