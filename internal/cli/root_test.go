package cli

import (
	"bytes"
	"encoding/json"
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
}

func TestCLIStatusJSONReturnsReport(t *testing.T) {
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
