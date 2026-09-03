package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/okfcli/okf/internal/cerr"
)

// --- issue #31: commands without flags must still reject flags as usage ---

func TestRejectFlags(t *testing.T) {
	if err := rejectFlags("index", []string{"./bundle"}); err != nil {
		t.Fatalf("plain path rejected: %v", err)
	}
	if err := rejectFlags("index", nil); err != nil {
		t.Fatalf("empty args rejected: %v", err)
	}
	for _, args := range [][]string{
		{"--nope", "./bundle"},
		{"./bundle", "--check"},
		{"-x"},
	} {
		err := rejectFlags("index", args)
		if err == nil {
			t.Fatalf("args %v: want usage error", args)
		}
		if err.Kind != cerr.KindUsage {
			t.Errorf("args %v: kind = %v, want usage", args, err.Kind)
		}
		if !strings.Contains(err.Message, "unknown index flag: -") {
			t.Errorf("args %v: message = %q", args, err.Message)
		}
	}
}

// --- issue #27: a non-concept .md is a validation error naming the file ---

func TestLoadBundle_MissingFrontmatterIsValidationError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.md"), "# Demo\n")
	writeFile(t, filepath.Join(dir, "README.md"), "# Demo project\n")

	_, err := loadBundle(dir)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Kind != cerr.KindValidation {
		t.Errorf("kind = %v, want validation", err.Kind)
	}
	for _, want := range []string{"README.md", "no YAML frontmatter"} {
		if !strings.Contains(err.Message, want) {
			t.Errorf("message %q does not mention %q", err.Message, want)
		}
	}
	if err.Hint == "" {
		t.Error("expected a hint explaining the frontmatter requirement")
	}
}

func TestLoadBundle_MissingDirIsIOError(t *testing.T) {
	_, err := loadBundle(filepath.Join(t.TempDir(), "nope"))
	if err == nil || err.Kind != cerr.KindIO {
		t.Fatalf("want io error, got %v", err)
	}
}

// --- end to end: the built binary's JSON envelope and exit code ---

var okfBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "okf-test-bin")
	if err != nil {
		panic(err)
	}
	okfBin = filepath.Join(dir, "okf")
	if runtime.GOOS == "windows" {
		okfBin += ".exe"
	}
	build := exec.Command("go", "build", "-o", okfBin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		panic("build okf: " + err.Error() + "\n" + string(out))
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

type envelope struct {
	Error struct {
		Kind    string `json:"kind"`
		Code    int    `json:"code"`
		Message string `json:"message"`
		Hint    string `json:"hint"`
	} `json:"error"`
}

func runOKF(t *testing.T, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(okfBin, args...) //nolint:gosec // test runs the binary built in TestMain
	out, err := cmd.Output()
	code := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("run %v: %v", args, err)
	}
	return string(out), code
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestE2E_UnknownFlagIsUsageError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.md"), "---\ntype: T\n---\n\nbody")
	for _, c := range []string{"index", "list", "graph", "search", "validate", "lint"} {
		out, code := runOKF(t, c, "--nope", dir)
		if code != cerr.ExitCodeUsage {
			t.Errorf("%s --nope: exit %d, want %d; out=%s", c, code, cerr.ExitCodeUsage, out)
		}
		var env envelope
		if err := json.Unmarshal([]byte(out), &env); err != nil {
			t.Errorf("%s --nope: bad json %q", c, out)
			continue
		}
		if env.Error.Kind != "usage" || env.Error.Code != 400 {
			t.Errorf("%s --nope: envelope %+v", c, env.Error)
		}
		if !strings.Contains(env.Error.Message, "--nope") {
			t.Errorf("%s --nope: message %q does not name the flag", c, env.Error.Message)
		}
	}
	for _, c := range []string{"show", "backlinks"} {
		out, code := runOKF(t, c, dir, "--nope")
		if code != cerr.ExitCodeUsage {
			t.Errorf("%s dir --nope: exit %d, want %d; out=%s", c, code, cerr.ExitCodeUsage, out)
		}
	}
	// Sanity: the bundle itself still loads.
	if _, code := runOKF(t, "list", dir); code != 0 {
		t.Errorf("list: exit %d, want 0", code)
	}
}

func TestE2E_MarkdownWithoutFrontmatterNamesFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.md"), "# Demo\n")
	writeFile(t, filepath.Join(dir, "README.md"), "# Demo project\n")

	out, code := runOKF(t, "list", dir)
	if code != cerr.ExitCodeValidation {
		t.Errorf("exit %d, want %d; out=%s", code, cerr.ExitCodeValidation, out)
	}
	var env envelope
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("bad json %q", out)
	}
	if env.Error.Kind != "validation" {
		t.Errorf("kind = %q, want validation", env.Error.Kind)
	}
	if !strings.Contains(env.Error.Message, "README.md") || !strings.Contains(env.Error.Message, "frontmatter") {
		t.Errorf("message %q does not name the file and reason", env.Error.Message)
	}
	if env.Error.Hint == "" {
		t.Error("expected a hint")
	}
}

func TestE2E_BrokenLinkDoesNotFailValidate(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.md"), "---\ntype: T\ntitle: A\ndescription: d\ntags: [x]\n---\n\nSee [b](/b.md).")
	out, code := runOKF(t, "validate", dir)
	if code != 0 {
		t.Fatalf("exit %d, want 0; out=%s", code, out)
	}
	var rep struct {
		Valid    bool `json:"valid"`
		Warnings int  `json:"warnings"`
		Errors   int  `json:"errors"`
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("bad json %q", out)
	}
	if !rep.Valid || rep.Errors != 0 || rep.Warnings != 1 {
		t.Errorf("report = %+v, want valid with one warning", rep)
	}
}

func TestE2E_IndexRefusesNonConceptMarkdown(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.md"), "---\ntype: T\ntitle: A\n---\n\nbody")
	writeFile(t, filepath.Join(dir, "README.md"), "# Demo project\n")

	out, code := runOKF(t, "index", dir)
	if code != cerr.ExitCodeValidation {
		t.Errorf("exit %d, want %d; out=%s", code, cerr.ExitCodeValidation, out)
	}
	if !strings.Contains(out, "README.md") {
		t.Errorf("output %q does not name the file", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "index.md")); err == nil {
		t.Error("index.md was written even though the bundle failed to load")
	}
}
