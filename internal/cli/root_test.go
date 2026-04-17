package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kanini/keystone-toolchain/internal/contract"
)

type cliResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

func runCLI(t *testing.T, args []string) cliResult {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	a := newApp(&stdout, &stderr)
	a.root.SetArgs(args)
	exitCode := executeApp(a)

	return cliResult{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}
}

func decodeEnvelope(t *testing.T, raw string) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("decode json: %v\nraw=%q", err, raw)
	}
	return payload
}

func TestCLIJSONPuritySuccess(t *testing.T) {
	res := runCLI(t, []string{"version", "--json"})
	if res.ExitCode != contract.ExitOK {
		t.Fatalf("unexpected exit code: got=%d want=%d\nstdout=%s\nstderr=%s", res.ExitCode, contract.ExitOK, res.Stdout, res.Stderr)
	}
	if strings.TrimSpace(res.Stderr) != "" {
		t.Fatalf("expected empty stderr, got %q", res.Stderr)
	}

	payload := decodeEnvelope(t, res.Stdout)
	if ok, _ := payload["ok"].(bool); !ok {
		t.Fatalf("expected ok=true payload: %#v", payload)
	}
	result, _ := payload["result"].(map[string]any)
	if name, _ := result["name"].(string); name != "kstoolchain" {
		t.Fatalf("unexpected result name: %#v", result)
	}
}

func TestCLISyncRejectsArgs(t *testing.T) {
	res := runCLI(t, []string{"sync", "extra", "--json"})
	if res.ExitCode != contract.ExitValidation {
		t.Fatalf("unexpected exit code: got=%d want=%d\nstdout=%s\nstderr=%s", res.ExitCode, contract.ExitValidation, res.Stdout, res.Stderr)
	}
	if strings.TrimSpace(res.Stderr) != "" {
		t.Fatalf("expected empty stderr, got %q", res.Stderr)
	}

	payload := decodeEnvelope(t, res.Stdout)
	if ok, _ := payload["ok"].(bool); ok {
		t.Fatalf("expected ok=false payload: %#v", payload)
	}
	errShape, _ := payload["error"].(map[string]any)
	if code, _ := errShape["code"].(string); code != contract.CodeArgsInvalid {
		t.Fatalf("unexpected error code: %#v", errShape)
	}
	if _, ok := payload["result"]; ok {
		t.Fatalf("argument failure must not emit a sync result body: %#v", payload)
	}
}

func TestCLIStatusJSONReturnsReport(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", "")
	res := runCLI(t, []string{"status", "--json"})
	if res.ExitCode != contract.ExitValidation {
		t.Fatalf("unexpected exit code: got=%d want=%d\nstdout=%s\nstderr=%s", res.ExitCode, contract.ExitValidation, res.Stdout, res.Stderr)
	}
	if strings.TrimSpace(res.Stderr) != "" {
		t.Fatalf("expected empty stderr, got %q", res.Stderr)
	}

	payload := decodeEnvelope(t, res.Stdout)
	if ok, _ := payload["ok"].(bool); !ok {
		t.Fatalf("expected ok=true payload: %#v", payload)
	}
	result, _ := payload["result"].(map[string]any)
	if schema, _ := result["schema"].(string); schema != "kstoolchain.status/v1alpha1" {
		t.Fatalf("unexpected status schema: %#v", result)
	}
}

func TestCLIStatusJSONIncludesOverlayMissingWarning(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", "")

	res := runCLI(t, []string{"status", "--json"})
	payload := decodeEnvelope(t, res.Stdout)
	warnings, _ := payload["warnings"].([]any)
	if len(warnings) == 0 {
		t.Fatalf("expected warnings in payload: %#v", payload)
	}
	warning, _ := warnings[0].(map[string]any)
	if code, _ := warning["code"].(string); code != contract.CodeOverlayMissing {
		t.Fatalf("unexpected warning code: %#v", warning)
	}
}

func TestCLISyncJSONFailsOnUnknownOverlayRow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", "")

	overlayPath := filepath.Join(home, ".keystone", "toolchain", "adapters.yaml")
	if err := os.MkdirAll(filepath.Dir(overlayPath), 0o755); err != nil {
		t.Fatalf("mkdir overlay dir: %v", err)
	}
	if err := os.WriteFile(overlayPath, []byte(`schema: kstoolchain.adapter-overlay/v1alpha1
repos:
  - repo_id: stale-repo
    repo_path: ~/git/stale-repo
`), 0o644); err != nil {
		t.Fatalf("write overlay: %v", err)
	}

	res := runCLI(t, []string{"sync", "--json"})
	if res.ExitCode != contract.ExitValidation {
		t.Fatalf("unexpected exit code: %d\nstdout=%s\nstderr=%s", res.ExitCode, res.Stdout, res.Stderr)
	}
	payload := decodeEnvelope(t, res.Stdout)
	if ok, _ := payload["ok"].(bool); ok {
		t.Fatalf("expected sync failure payload: %#v", payload)
	}
	errShape, _ := payload["error"].(map[string]any)
	if code, _ := errShape["code"].(string); code != contract.CodeOverlayUnknown {
		t.Fatalf("unexpected error code: %#v", errShape)
	}
	if _, ok := payload["result"]; ok {
		t.Fatalf("command-level sync failure must not emit a result body: %#v", payload)
	}
	if _, err := os.Stat(filepath.Join(home, ".keystone", "toolchain", "state", "current.json")); !os.IsNotExist(err) {
		t.Fatalf("expected no current.json write on overlay failure, got err=%v", err)
	}
}

func TestCLISyncJSONFailsClosedOnInvalidAttemptArtifact(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", "")

	stateDir := filepath.Join(home, ".keystone", "toolchain", "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "attempt.json"), []byte("{invalid\n"), 0o644); err != nil {
		t.Fatalf("write invalid attempt artifact: %v", err)
	}

	res := runCLI(t, []string{"sync", "--json"})
	if res.ExitCode != contract.ExitValidation {
		t.Fatalf("unexpected exit code: %d\nstdout=%s\nstderr=%s", res.ExitCode, res.Stdout, res.Stderr)
	}
	payload := decodeEnvelope(t, res.Stdout)
	if ok, _ := payload["ok"].(bool); ok {
		t.Fatalf("expected sync failure payload: %#v", payload)
	}
	errShape, _ := payload["error"].(map[string]any)
	if code, _ := errShape["code"].(string); code != contract.CodeConfigInvalid {
		t.Fatalf("unexpected error code: %#v", errShape)
	}
	if _, ok := payload["result"]; ok {
		t.Fatalf("invalid attempt artifact must not emit a sync result body: %#v", payload)
	}
	if _, err := os.Stat(filepath.Join(stateDir, "current.json")); !os.IsNotExist(err) {
		t.Fatalf("invalid attempt artifact must block current.json writes, got err=%v", err)
	}
}

func TestCLISyncJSONCompletedWithBlockersUsesResultBearingNonSuccess(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", "")

	res := runCLI(t, []string{"sync", "--json"})
	if res.ExitCode != contract.ExitReadySetBlocked {
		t.Fatalf("unexpected exit code: got=%d want=%d\nstdout=%s\nstderr=%s", res.ExitCode, contract.ExitReadySetBlocked, res.Stdout, res.Stderr)
	}
	payload := decodeEnvelope(t, res.Stdout)
	if ok, _ := payload["ok"].(bool); ok {
		t.Fatalf("completed_with_blockers must be ok=false: %#v", payload)
	}
	if _, ok := payload["error"]; ok {
		t.Fatalf("completed_with_blockers must not emit a top-level error: %#v", payload)
	}
	result, _ := payload["result"].(map[string]any)
	if schema, _ := result["schema"].(string); schema != "kstoolchain.sync-report/v1alpha1" {
		t.Fatalf("unexpected sync schema: %#v", result)
	}
	if outcome, _ := result["outcome"].(string); outcome != "completed_with_blockers" {
		t.Fatalf("unexpected sync outcome: %#v", result)
	}
	finalStatus, _ := result["final_status"].(map[string]any)
	if schema, _ := finalStatus["schema"].(string); schema != "kstoolchain.status/v1alpha1" {
		t.Fatalf("unexpected nested final_status schema: %#v", finalStatus)
	}
}

func TestCLISyncJSONSchemaIsStableUnderVerbose(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", "")

	res := runCLI(t, []string{"sync", "--json", "--verbose"})
	payload := decodeEnvelope(t, res.Stdout)
	result, _ := payload["result"].(map[string]any)
	if schema, _ := result["schema"].(string); schema != "kstoolchain.sync-report/v1alpha1" {
		t.Fatalf("unexpected sync schema with --verbose: %#v", result)
	}
	if _, ok := result["final_status"]; !ok {
		t.Fatalf("expected final_status in sync JSON with --verbose: %#v", result)
	}
}

func TestCLISyncTextDefaultDiffersFromVerbose(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", "")

	defaultRes := runCLI(t, []string{"sync"})
	if !strings.Contains(defaultRes.Stdout, "Sync: completed_with_blockers") {
		t.Fatalf("unexpected default sync text: %q", defaultRes.Stdout)
	}
	if strings.Contains(defaultRes.Stdout, "Suite:") {
		t.Fatalf("default sync text must not reuse the broad status dump: %q", defaultRes.Stdout)
	}

	verboseRes := runCLI(t, []string{"sync", "--verbose"})
	if !strings.Contains(verboseRes.Stdout, "Final ready-set detail:") {
		t.Fatalf("expected verbose sync text to widen: %q", verboseRes.Stdout)
	}
	if !strings.Contains(verboseRes.Stdout, "Suite:") {
		t.Fatalf("expected verbose sync text to include status detail: %q", verboseRes.Stdout)
	}
}

func TestCLIInitRejectsJSON(t *testing.T) {
	res := runCLI(t, []string{"init", "--json"})
	if res.ExitCode != contract.ExitValidation {
		t.Fatalf("unexpected exit code: got=%d want=%d\nstdout=%s\nstderr=%s", res.ExitCode, contract.ExitValidation, res.Stdout, res.Stderr)
	}
	if strings.TrimSpace(res.Stderr) != "" {
		t.Fatalf("expected empty stderr, got %q", res.Stderr)
	}

	payload := decodeEnvelope(t, res.Stdout)
	if ok, _ := payload["ok"].(bool); ok {
		t.Fatalf("expected init json rejection payload: %#v", payload)
	}
	errShape, _ := payload["error"].(map[string]any)
	if code, _ := errShape["code"].(string); code != contract.CodeArgsInvalid {
		t.Fatalf("unexpected error code: %#v", errShape)
	}
}

func TestCLITextVersion(t *testing.T) {
	res := runCLI(t, []string{"version"})
	if res.ExitCode != contract.ExitOK {
		t.Fatalf("unexpected exit code: got=%d want=%d\nstdout=%s\nstderr=%s", res.ExitCode, contract.ExitOK, res.Stdout, res.Stderr)
	}
	if !strings.Contains(res.Stdout, "kstoolchain version") {
		t.Fatalf("expected version line, got %q", res.Stdout)
	}
}

func TestCLIBadFormatFailsValidation(t *testing.T) {
	res := runCLI(t, []string{"version", "--format", "xml"})
	if res.ExitCode != contract.ExitValidation {
		t.Fatalf("unexpected exit code: got=%d want=%d\nstdout=%s\nstderr=%s", res.ExitCode, contract.ExitValidation, res.Stdout, res.Stderr)
	}
	if !strings.Contains(res.Stdout, "FAIL "+contract.CodeArgsInvalid) {
		t.Fatalf("expected args invalid failure, got %q", res.Stdout)
	}
}
