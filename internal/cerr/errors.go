// Package cerr provides structured CLI errors with stable exit codes and
// JSON envelopes for programmatic consumption.
package cerr

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Kind categorises an error for routing and exit-code mapping.
type Kind int

// Error kinds.
const (
	KindOK Kind = iota
	KindValidation
	KindIO
	KindInternal
	KindUsage
)

// String returns the lowercase tag used in JSON envelopes.
func (k Kind) String() string {
	switch k {
	case KindOK:
		return "ok"
	case KindValidation:
		return "validation"
	case KindIO:
		return "io"
	case KindInternal:
		return "internal"
	case KindUsage:
		return "usage"
	default:
		return "unknown"
	}
}

// Stable exit codes.
const (
	ExitCodeOK         = 0
	ExitCodeValidation = 1
	ExitCodeIO         = 2
	ExitCodeInternal   = 3
	ExitCodeUsage      = 4
)

// ExitCodeDoc describes a single exit code for help/docs rendering.
type ExitCodeDoc struct {
	Code        int    `json:"code"`
	Description string `json:"description"`
}

// ExitCodeDocs enumerates every exit code the CLI may emit, in stable order.
var ExitCodeDocs = []ExitCodeDoc{
	{ExitCodeOK, "success"},
	{ExitCodeValidation, "validation error (spec violation, broken link, bad input)"},
	{ExitCodeIO, "filesystem or I/O error"},
	{ExitCodeInternal, "internal error (unexpected)"},
	{ExitCodeUsage, "usage error (missing args, unknown command)"},
}

// Error is a structured CLI error.
type Error struct {
	Kind    Kind
	Code    int
	Reason  string
	Message string
	Hint    string
	Cause   error
}

// Error implements the error interface.
func (e *Error) Error() string { return e.Message }

// Unwrap exposes the underlying cause for errors.Is / errors.As.
func (e *Error) Unwrap() error { return e.Cause }

// ExitCode maps Kind to a stable process exit code.
func (e *Error) ExitCode() int {
	switch e.Kind {
	case KindValidation:
		return ExitCodeValidation
	case KindIO:
		return ExitCodeIO
	case KindInternal:
		return ExitCodeInternal
	case KindUsage:
		return ExitCodeUsage
	default:
		return ExitCodeInternal
	}
}

// ToEnvelope returns a JSON-ready map describing the error.
func (e *Error) ToEnvelope() map[string]any {
	inner := map[string]any{
		"kind":    e.Kind.String(),
		"code":    e.Code,
		"reason":  e.Reason,
		"message": e.Message,
	}
	if e.Hint != "" {
		inner["hint"] = e.Hint
	}
	return map[string]any{"error": inner}
}

// ToJSON returns the indented JSON envelope.
func (e *Error) ToJSON() ([]byte, error) {
	return json.MarshalIndent(e.ToEnvelope(), "", "  ")
}

func format(msg string, args ...any) string {
	if len(args) == 0 {
		return msg
	}
	return fmt.Sprintf(msg, args...)
}

// Validation builds a validation error.
func Validation(msg string, args ...any) *Error {
	return &Error{
		Kind:    KindValidation,
		Code:    400,
		Reason:  "validationError",
		Message: format(msg, args...),
	}
}

// IO builds an I/O error wrapping cause.
func IO(cause error, msg string, args ...any) *Error {
	return &Error{
		Kind:    KindIO,
		Code:    500,
		Reason:  "ioError",
		Message: format(msg, args...),
		Cause:   cause,
	}
}

// Usage builds a usage error.
func Usage(msg string, args ...any) *Error {
	return &Error{
		Kind:    KindUsage,
		Code:    400,
		Reason:  "usageError",
		Message: format(msg, args...),
	}
}

// Internal builds an internal error wrapping cause.
func Internal(cause error, msg string, args ...any) *Error {
	return &Error{
		Kind:    KindInternal,
		Code:    500,
		Reason:  "internalError",
		Message: format(msg, args...),
		Cause:   cause,
	}
}

// From returns err as *Error. If err is already *Error it is returned as-is;
// otherwise it is wrapped as an internal error. Returns nil for nil.
func From(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return Internal(err, "%s", err.Error())
}
