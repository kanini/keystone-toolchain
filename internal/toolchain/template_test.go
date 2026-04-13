package toolchain

import "testing"

func TestExpandCommandArgs(t *testing.T) {
	args, appErr := expandCommandArgs(
		[]string{"make", "build-local", "BIN_DIR={{stage_bin}}", "ROOT={{stage_bin_parent}}"},
		templateVars{stageBin: "/tmp/stage/bin", stageBinParent: "/tmp/stage"},
	)
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if got := args[2]; got != "BIN_DIR=/tmp/stage/bin" {
		t.Fatalf("unexpected stage bin expansion: %q", got)
	}
	if got := args[3]; got != "ROOT=/tmp/stage" {
		t.Fatalf("unexpected stage bin parent expansion: %q", got)
	}
}

func TestExpandCommandArgsRejectsUnknownToken(t *testing.T) {
	_, appErr := expandCommandArgs([]string{"echo", "{{unknown}}"}, templateVars{})
	if appErr == nil {
		t.Fatal("expected unknown token error")
	}
}
