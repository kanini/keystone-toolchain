package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kanini/keystone-toolchain/internal/contract"
)

func TestLoadConfigDefaults(t *testing.T) {
	home := t.TempDir()

	cfg, cfgPath, err := LoadConfig(home, GlobalOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ManagedBinDir != filepath.Join(home, ".keystone", "bin") {
		t.Fatalf("unexpected managed bin dir: %s", cfg.ManagedBinDir)
	}
	if cfg.StateDir != filepath.Join(home, ".keystone", "toolchain") {
		t.Fatalf("unexpected state dir: %s", cfg.StateDir)
	}
	if cfgPath != filepath.Join(home, ".keystone", "toolchain", "config.yaml") {
		t.Fatalf("unexpected config path: %s", cfgPath)
	}
}

func TestLoadConfigMergesFileEnvAndFlags(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".keystone", "toolchain", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := "managed_bin_dir: ~/from-file/bin\nstate_dir: ~/from-file/state\n"
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("KSTOOLCHAIN_MANAGED_BIN_DIR", filepath.Join(home, "from-env", "bin"))

	cfg, _, err := LoadConfig(home, GlobalOptions{
		StateDir: filepath.Join(home, "from-flag", "state"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ManagedBinDir != filepath.Join(home, "from-env", "bin") {
		t.Fatalf("unexpected managed bin dir: %s", cfg.ManagedBinDir)
	}
	if cfg.StateDir != filepath.Join(home, "from-flag", "state") {
		t.Fatalf("unexpected state dir: %s", cfg.StateDir)
	}
}

func TestBuildContextRejectsBadFormat(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	_, err := BuildContext(GlobalOptions{Format: "xml"})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Code != contract.CodeArgsInvalid {
		t.Fatalf("unexpected code: %s", err.Code)
	}
}
