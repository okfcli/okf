package cerr

import (
	"errors"
	"testing"
)

func TestValidationExitCode(t *testing.T) {
	e := Validation("bad input")
	if e.ExitCode() != ExitCodeValidation {
		t.Errorf("exit code = %d, want %d", e.ExitCode(), ExitCodeValidation)
	}
	if e.Kind != KindValidation {
		t.Errorf("kind = %v, want %v", e.Kind, KindValidation)
	}
}

func TestIOExitCode(t *testing.T) {
	cause := errors.New("disk full")
	e := IO(cause, "cannot read file")
	if e.ExitCode() != ExitCodeIO {
		t.Errorf("exit code = %d, want %d", e.ExitCode(), ExitCodeIO)
	}
	if !errors.Is(e, cause) {
		t.Error("Unwrap did not return cause")
	}
}

func TestInternalExitCode(t *testing.T) {
	e := Internal(errors.New("boom"), "unexpected")
	if e.ExitCode() != ExitCodeInternal {
		t.Errorf("exit code = %d, want %d", e.ExitCode(), ExitCodeInternal)
	}
}

func TestFromWrapsUnknown(t *testing.T) {
	plain := errors.New("plain error")
	e := From(plain)
	if e.Kind != KindInternal {
		t.Errorf("kind = %v, want %v", e.Kind, KindInternal)
	}
}

func TestFromReturnsExisting(t *testing.T) {
	original := Validation("already structured")
	e := From(original)
	if e != original {
		t.Error("From should return existing *Error as-is")
	}
}

func TestFromNil(t *testing.T) {
	if From(nil) != nil {
		t.Error("From(nil) should return nil")
	}
}

func TestToEnvelopeOmitsEmptyHint(t *testing.T) {
	e := Validation("bad")
	env := e.ToEnvelope()
	inner := env["error"].(map[string]any)
	if _, ok := inner["hint"]; ok {
		t.Error("hint should be omitted when empty")
	}
}

func TestToEnvelopeIncludesHint(t *testing.T) {
	e := &Error{Kind: KindValidation, Code: 400, Reason: "x", Message: "bad", Hint: "try again"}
	env := e.ToEnvelope()
	inner := env["error"].(map[string]any)
	if inner["hint"] != "try again" {
		t.Errorf("hint = %v, want 'try again'", inner["hint"])
	}
}

func TestKindString(t *testing.T) {
	tests := []struct {
		kind Kind
		want string
	}{
		{KindOK, "ok"},
		{KindValidation, "validation"},
		{KindIO, "io"},
		{KindInternal, "internal"},
		{KindUsage, "usage"},
	}
	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.want {
			t.Errorf("Kind(%d).String() = %q, want %q", tt.kind, got, tt.want)
		}
	}
}
