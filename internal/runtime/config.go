package runtime

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/kanini/keystone-toolchain/internal/contract"
)

type Config struct {
	ManagedBinDir string
	StateDir      string
}

type rawConfig struct {
	ManagedBinDir string `yaml:"managed_bin_dir"`
	StateDir      string `yaml:"state_dir"`
}

type GlobalOptions struct {
	ConfigPath    string
	Format        string
	JSON          bool
	ManagedBinDir string
	StateDir      string
}

func LoadConfig(home string, opts GlobalOptions) (Config, string, *contract.AppError) {
	cfg := Config{
		ManagedBinDir: filepath.Join(home, ".keystone", "bin"),
		StateDir:      filepath.Join(home, ".keystone", "toolchain"),
	}

	configPath, err := resolveConfigPath(home, opts.ConfigPath)
	if err != nil {
		return Config{}, "", contract.Validation(contract.CodeConfigInvalid, "Could not resolve config path.", "Set --config to a valid path.", contract.Detail{Name: "config", Value: opts.ConfigPath})
	}

	if configPath != "" {
		rawBytes, readErr := os.ReadFile(configPath)
		if readErr == nil {
			var parsed rawConfig
			if err := yaml.Unmarshal(rawBytes, &parsed); err != nil {
				return Config{}, "", contract.Validation(contract.CodeConfigInvalid, "Config file is not valid YAML.", "Fix the file and retry.", contract.Detail{Name: "path", Value: configPath})
			}
			if strings.TrimSpace(parsed.ManagedBinDir) != "" {
				cfg.ManagedBinDir = parsed.ManagedBinDir
			}
			if strings.TrimSpace(parsed.StateDir) != "" {
				cfg.StateDir = parsed.StateDir
			}
		} else if !os.IsNotExist(readErr) {
			return Config{}, "", contract.Infra(contract.CodeIOError, "Could not read config file.", "Check file permissions and retry.", readErr, contract.Detail{Name: "path", Value: configPath})
		}
	}

	if env := strings.TrimSpace(os.Getenv("KSTOOLCHAIN_MANAGED_BIN_DIR")); env != "" {
		cfg.ManagedBinDir = env
	}
	if env := strings.TrimSpace(os.Getenv("KSTOOLCHAIN_STATE_DIR")); env != "" {
		cfg.StateDir = env
	}

	if strings.TrimSpace(opts.ManagedBinDir) != "" {
		cfg.ManagedBinDir = opts.ManagedBinDir
	}
	if strings.TrimSpace(opts.StateDir) != "" {
		cfg.StateDir = opts.StateDir
	}

	cfg.ManagedBinDir, err = normalizePath(cfg.ManagedBinDir, home)
	if err != nil {
		return Config{}, "", contract.Validation(contract.CodeConfigInvalid, "managed_bin_dir must be a valid path.", "Set managed_bin_dir to a usable path.", contract.Detail{Name: "managed_bin_dir", Value: cfg.ManagedBinDir})
	}
	cfg.StateDir, err = normalizePath(cfg.StateDir, home)
	if err != nil {
		return Config{}, "", contract.Validation(contract.CodeConfigInvalid, "state_dir must be a valid path.", "Set state_dir to a usable path.", contract.Detail{Name: "state_dir", Value: cfg.StateDir})
	}
	if cfg.ManagedBinDir == "" || cfg.StateDir == "" {
		return Config{}, "", contract.Validation(contract.CodeConfigInvalid, "managed_bin_dir and state_dir are required.", "Set both paths in config, env, or flags.")
	}

	return cfg, configPath, nil
}

func resolveConfigPath(home, explicit string) (string, error) {
	if strings.TrimSpace(explicit) == "" {
		return filepath.Join(home, ".keystone", "toolchain", "config.yaml"), nil
	}
	return normalizePath(explicit, home)
}

func normalizePath(rawPath, home string) (string, error) {
	path := strings.TrimSpace(rawPath)
	switch {
	case path == "":
		return "", nil
	case path == "~":
		path = home
	case strings.HasPrefix(path, "~/"):
		path = filepath.Join(home, path[2:])
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	return filepath.Abs(path)
}
