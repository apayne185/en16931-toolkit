// Black-box tests for the en16931 CLI binary.
//
// main() calls os.Exit on every error path, which can't be exercised
// in-process without killing the test binary, so these tests build the
// real binary once and drive it via exec.Command, asserting on stdout,
// stderr, and exit code exactly as a user would observe them.
package main_test

import (
	"bytes"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// binPath is built once per test run by TestMain and reused by every test.
var binPath string

// coverDir collects GOCOVERDIR profiles from every subprocess run, so
// running these tests with `go test -cover` reports real coverage for
// main.go even though it only ever executes in a child process.
var coverDir string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "en16931-cli-test")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	binPath = filepath.Join(dir, "en16931")
	build := exec.Command("go", "build", "-cover", "-o", binPath, ".")
	if out, err := build.CombinedOutput(); err != nil {
		panic("build failed: " + err.Error() + "\n" + string(out))
	}

	coverDir = os.Getenv("GOCOVERDIR")
	if coverDir == "" {
		coverDir, err = os.MkdirTemp("", "en16931-cli-cover")
		if err != nil {
			panic(err)
		}
		defer os.RemoveAll(coverDir)
	}

	os.Exit(m.Run())
}

func run(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Env = append(os.Environ(), "GOCOVERDIR="+coverDir)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run binary: %v", err)
		}
	}
	return outBuf.String(), errBuf.String(), exitCode
}

func fixture(name string) string {
	return filepath.Join("..", "..", "examples", name)
}

func TestNoArgs(t *testing.T) {
	_, stderr, code := run(t)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr, "Usage:") {
		t.Errorf("expected usage text on stderr, got %q", stderr)
	}
}

func TestUnknownCommand(t *testing.T) {
	_, stderr, code := run(t, "frobnicate")
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr, `unknown command "frobnicate"`) {
		t.Errorf("expected unknown command message, got %q", stderr)
	}
}

func TestValidate_Pass(t *testing.T) {
	stdout, stderr, code := run(t, "validate", fixture("simple_invoice.json"))
	if code != 0 {
		t.Errorf("expected exit code 0, got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "✓") || !strings.Contains(stdout, "INV-2024-001") {
		t.Errorf("expected success message with invoice number, got %q", stdout)
	}
	if !strings.Contains(stdout, "rules checked") {
		t.Errorf("expected rule count in output, got %q", stdout)
	}
}

func TestValidate_NoFileArg(t *testing.T) {
	_, stderr, code := run(t, "validate")
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr, "usage: en16931 validate") {
		t.Errorf("expected validate usage message, got %q", stderr)
	}
}

func TestValidate_MissingFile(t *testing.T) {
	_, stderr, code := run(t, "validate", "does-not-exist.json")
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr, "cannot open") {
		t.Errorf("expected 'cannot open' error, got %q", stderr)
	}
}

func TestValidate_InvalidJSON(t *testing.T) {
	badFile := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(badFile, []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := run(t, "validate", badFile)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr, "invalid JSON") {
		t.Errorf("expected 'invalid JSON' error, got %q", stderr)
	}
}

func TestValidate_RuleViolations(t *testing.T) {
	invalidInvoice := filepath.Join(t.TempDir(), "invalid.json")
	body := `{"number":"","issue_date":"2024-06-01","type_code":"380","currency":"EUR",
		"buyer_reference":"PO-1",
		"seller":{"name":"S","vat_id":"ESB12345674","address":{"country":"ES"}},
		"buyer":{"name":"B","address":{"country":"DE"}},
		"vat_breakdown":[{"category":"S","rate":21,"taxable_amount":100,"tax_amount":21}],
		"totals":{"line_net_total":100,"tax_exclusive_amount":100,"tax_amount":21,"tax_inclusive_amount":121,"payable_amount":121},
		"lines":[{"id":"1","quantity":1,"quantity_unit":"C62","net_amount":100,"vat":{"category":"S","rate":21},"item":{"name":"Widget"},"price":{"amount":100}}]}`
	if err := os.WriteFile(invalidInvoice, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := run(t, "validate", invalidInvoice)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr, "BR-2") {
		t.Errorf("expected BR-2 (missing invoice number) in output, got %q", stderr)
	}
	if !strings.Contains(stderr, "Validation failed") {
		t.Errorf("expected 'Validation failed' summary, got %q", stderr)
	}
}

func TestRender_Stdout(t *testing.T) {
	stdout, stderr, code := run(t, "render", fixture("simple_invoice.json"))
	if code != 0 {
		t.Errorf("expected exit code 0, got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "<Invoice") {
		t.Errorf("expected UBL XML on stdout, got %q", stdout)
	}
	if !strings.Contains(stdout, "INV-2024-001") {
		t.Errorf("expected invoice number in rendered XML, got %q", stdout)
	}
}

func TestRender_ToFile(t *testing.T) {
	outFile := filepath.Join(t.TempDir(), "out.xml")
	stdout, stderr, code := run(t, "render", fixture("simple_invoice.json"), "-o", outFile)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "Wrote UBL 2.1 invoice") {
		t.Errorf("expected write confirmation, got %q", stdout)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("expected output file to exist: %v", err)
	}
	if !strings.Contains(string(data), "<Invoice") {
		t.Errorf("expected UBL XML in output file, got %q", data)
	}
}

func TestRender_InvalidInvoiceRefusesToRender(t *testing.T) {
	invalidInvoice := filepath.Join(t.TempDir(), "invalid.json")
	body := `{"number":"","currency":"EUR","type_code":"380","issue_date":"2024-01-01","buyer_reference":"PO-1","seller":{"name":"S","vat_id":"ESB12345674","address":{"country":"ES"}},"buyer":{"name":"B","address":{"country":"DE"}},"vat_breakdown":[],"totals":{"line_net_total":0,"tax_exclusive_amount":0,"tax_amount":0,"tax_inclusive_amount":0,"payable_amount":0},"lines":[]}`
	if err := os.WriteFile(invalidInvoice, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := run(t, "render", invalidInvoice)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr, "Cannot render") {
		t.Errorf("expected 'Cannot render' message, got %q", stderr)
	}
}

func TestRender_NoFileArg(t *testing.T) {
	_, stderr, code := run(t, "render")
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr, "usage: en16931 render") {
		t.Errorf("expected render usage message, got %q", stderr)
	}
}

func TestVerifactu_FirstInSeries(t *testing.T) {
	stdout, stderr, code := run(t, "verifactu", fixture("simple_invoice.json"))
	if code != 0 {
		t.Errorf("expected exit code 0, got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "Veri*Factu") {
		t.Errorf("expected Veri*Factu confirmation, got %q", stdout)
	}
	if !strings.Contains(stdout, "Huella:") {
		t.Errorf("expected hash (Huella) in output, got %q", stdout)
	}
	if !strings.Contains(stdout, "First invoice in series") {
		t.Errorf("expected first-in-series note when no prev-hash given, got %q", stdout)
	}
}

func TestVerifactu_ChainedInvoice(t *testing.T) {
	stdout, stderr, code := run(t, "verifactu", fixture("simple_invoice.json"),
		"-prev-hash", strings.Repeat("A", 64),
		"-prev-timestamp", "01-06-2024 09:00:00")
	if code != 0 {
		t.Errorf("expected exit code 0, got %d (stderr: %s)", code, stderr)
	}
	if strings.Contains(stdout, "First invoice in series") {
		t.Errorf("did not expect first-in-series note when prev-hash given, got %q", stdout)
	}
}

func TestVerifactu_FailsSpainRules(t *testing.T) {
	// Buyer/seller are both non-Spanish, so ES-01/ES-02 fire.
	invalidInvoice := filepath.Join(t.TempDir(), "invalid.json")
	body := `{"number":"INV-1","issue_date":"2024-06-01","type_code":"380","currency":"EUR",
		"buyer_reference":"PO-1",
		"seller":{"name":"S","vat_id":"DE123456789","address":{"country":"DE"}},
		"buyer":{"name":"B","address":{"country":"FR"}},
		"vat_breakdown":[{"category":"S","rate":21,"taxable_amount":100,"tax_amount":21}],
		"totals":{"line_net_total":100,"tax_exclusive_amount":100,"tax_amount":21,"tax_inclusive_amount":121,"payable_amount":121},
		"lines":[{"id":"1","quantity":1,"quantity_unit":"C62","net_amount":100,"vat":{"category":"S","rate":21},"item":{"name":"Widget"},"price":{"amount":100}}]}`
	if err := os.WriteFile(invalidInvoice, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := run(t, "verifactu", invalidInvoice)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr, "Veri*Factu validation failed") {
		t.Errorf("expected Veri*Factu failure message, got %q", stderr)
	}
}

func TestVerifactu_NoFileArg(t *testing.T) {
	_, stderr, code := run(t, "verifactu")
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr, "usage: en16931 verifactu") {
		t.Errorf("expected verifactu usage message, got %q", stderr)
	}
}

func TestServe_StartsAndServesHealthz(t *testing.T) {
	cmd := exec.Command(binPath, "serve", "-addr", ":18080")
	cmd.Env = append(os.Environ(), "GOCOVERDIR="+coverDir)
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	// SIGTERM (not Kill) so Listen's graceful-shutdown path runs and the
	// covered binary gets a chance to flush its GOCOVERDIR profile on exit.
	defer func() {
		_ = cmd.Process.Signal(os.Interrupt)
		_ = cmd.Wait()
	}()

	var resp *http.Response
	var err error
	for i := 0; i < 50; i++ {
		resp, err = http.Get("http://127.0.0.1:18080/healthz")
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("server did not become healthy: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 from /healthz, got %d", resp.StatusCode)
	}
}
