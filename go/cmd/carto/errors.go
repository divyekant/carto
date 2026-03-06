package main

// errors.go — typed CLI errors with error codes and exit codes.
//
// Every error returned from a command can be classified into an error code
// (for JSON envelopes) and an exit code (for the process). Commands that
// use runWithEnvelope get this automatically; other callers can use
// toCliError(err) to extract/wrap any error.

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
)

// ─── Error code constants ─────────────────────────────────────────────────
// These appear in the JSON error envelope's "code" field.

const (
	ErrCodeGeneral    = "GENERAL_ERROR"
	ErrCodeNotFound   = "NOT_FOUND"
	ErrCodeConnection = "CONNECTION_ERROR"
	ErrCodeAuth       = "AUTH_FAILURE"
	ErrCodeConfig     = "CONFIG_ERROR"
)

// ─── Exit code constant ──────────────────────────────────────────────────
// ExitOK(0), ExitErr(1), ExitConfig(3), ExitConnRefused(4),
// ExitAuthFailure(5) are already defined in helpers.go.

const (
	ExitNotFound = 2 // resource not found
)

// ─── cliError type ────────────────────────────────────────────────────────

// cliError is a structured error that carries both a machine-readable
// error code (for JSON output) and a process exit code.
type cliError struct {
	msg  string
	code string
	exit int
}

// Error implements the error interface.
func (e *cliError) Error() string { return e.msg }

// ─── Constructor functions ────────────────────────────────────────────────

func newConnectionError(msg string) error {
	return &cliError{msg: msg, code: ErrCodeConnection, exit: ExitConnRefused}
}

func newAuthError(msg string) error {
	return &cliError{msg: msg, code: ErrCodeAuth, exit: ExitAuthFailure}
}

func newNotFoundError(msg string) error {
	return &cliError{msg: msg, code: ErrCodeNotFound, exit: ExitNotFound}
}

func newConfigError(msg string) error {
	return &cliError{msg: msg, code: ErrCodeConfig, exit: ExitConfig}
}

// ─── Classifier ───────────────────────────────────────────────────────────

// toCliError extracts a *cliError from err using errors.As.
// If err is not a cliError, it wraps it as a GENERAL_ERROR.
func toCliError(err error) *cliError {
	var ce *cliError
	if errors.As(err, &ce) {
		return ce
	}
	return &cliError{
		msg:  err.Error(),
		code: ErrCodeGeneral,
		exit: ExitErr,
	}
}

// ─── runWithEnvelope ──────────────────────────────────────────────────────

// runWithEnvelope executes fn, writes the result via writeEnvelopeHuman,
// logs an audit event, and exits with the appropriate code on error.
//
// This is the standard "run a command" wrapper that future commands should
// adopt. It handles JSON envelopes, human output, audit logging, and exit
// codes in one place.
func runWithEnvelope(cmd *cobra.Command, humanFn func(data any), fn func() (any, error)) {
	data, err := fn()

	if err != nil {
		ce := toCliError(err)

		// Human mode: print a coloured error line.
		if !isJSONMode(cmd) {
			printError("%s", ce.msg)
		}

		// Write the error envelope (JSON mode) or no-op (human mode already handled).
		writeEnvelopeHuman(cmd, nil, err, nil)

		logAuditEvent(cmd, "error", ce.msg, nil)
		os.Exit(ce.exit)
	}

	writeEnvelopeHuman(cmd, data, nil, func() {
		if humanFn != nil {
			humanFn(data)
		}
	})

	logAuditEvent(cmd, "ok", "", nil)
}
