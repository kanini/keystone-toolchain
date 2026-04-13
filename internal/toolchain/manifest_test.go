package toolchain

import (
	"testing"

	"github.com/kanini/keystone-toolchain/internal/runtime"
)

func TestLoadManifest(t *testing.T) {
	ctx := &runtime.Context{HomeDir: t.TempDir()}
	manifest, err := LoadManifest(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if manifest.Schema == "" {
		t.Fatal("expected schema")
	}
	if len(manifest.Repos) < 4 {
		t.Fatalf("expected repo entries, got %d", len(manifest.Repos))
	}
	if manifest.ManagedBinDir == "" {
		t.Fatal("expected managed bin dir")
	}
}
