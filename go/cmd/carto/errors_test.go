package main

import (
	"errors"
	"testing"
)

// =========================================================================
// Constructor tests
// =========================================================================

func TestNewConnectionError(t *testing.T) {
	err := newConnectionError("server unreachable")

	var ce *cliError
	if !errors.As(err, &ce) {
		t.Fatal("newConnectionError must return a *cliError")
	}
	if ce.code != ErrCodeConnection {
		t.Errorf("expected code %q, got %q", ErrCodeConnection, ce.code)
	}
	if ce.exit != ExitConnRefused {
		t.Errorf("expected exit %d, got %d", ExitConnRefused, ce.exit)
	}
	if ce.Error() != "server unreachable" {
		t.Errorf("expected msg %q, got %q", "server unreachable", ce.Error())
	}
}

func TestNewAuthError(t *testing.T) {
	err := newAuthError("bad token")

	var ce *cliError
	if !errors.As(err, &ce) {
		t.Fatal("newAuthError must return a *cliError")
	}
	if ce.code != ErrCodeAuth {
		t.Errorf("expected code %q, got %q", ErrCodeAuth, ce.code)
	}
	if ce.exit != ExitAuthFailure {
		t.Errorf("expected exit %d, got %d", ExitAuthFailure, ce.exit)
	}
}

func TestNewNotFoundError(t *testing.T) {
	err := newNotFoundError("project missing")

	var ce *cliError
	if !errors.As(err, &ce) {
		t.Fatal("newNotFoundError must return a *cliError")
	}
	if ce.code != ErrCodeNotFound {
		t.Errorf("expected code %q, got %q", ErrCodeNotFound, ce.code)
	}
	if ce.exit != ExitNotFound {
		t.Errorf("expected exit %d, got %d", ExitNotFound, ce.exit)
	}
}

func TestNewConfigError(t *testing.T) {
	err := newConfigError("missing key")

	var ce *cliError
	if !errors.As(err, &ce) {
		t.Fatal("newConfigError must return a *cliError")
	}
	if ce.code != ErrCodeConfig {
		t.Errorf("expected code %q, got %q", ErrCodeConfig, ce.code)
	}
	if ce.exit != ExitConfig {
		t.Errorf("expected exit %d, got %d", ExitConfig, ce.exit)
	}
}

// =========================================================================
// Classifier tests
// =========================================================================

func TestClassifyError_Connection(t *testing.T) {
	err := newConnectionError("timeout")
	ce := toCliError(err)

	if ce.code != ErrCodeConnection {
		t.Errorf("expected code %q, got %q", ErrCodeConnection, ce.code)
	}
	if ce.exit != ExitConnRefused {
		t.Errorf("expected exit %d, got %d", ExitConnRefused, ce.exit)
	}
}

func TestClassifyError_Auth(t *testing.T) {
	err := newAuthError("forbidden")
	ce := toCliError(err)

	if ce.code != ErrCodeAuth {
		t.Errorf("expected code %q, got %q", ErrCodeAuth, ce.code)
	}
	if ce.exit != ExitAuthFailure {
		t.Errorf("expected exit %d, got %d", ExitAuthFailure, ce.exit)
	}
}

func TestClassifyError_NotFound(t *testing.T) {
	err := newNotFoundError("no such project")
	ce := toCliError(err)

	if ce.code != ErrCodeNotFound {
		t.Errorf("expected code %q, got %q", ErrCodeNotFound, ce.code)
	}
	if ce.exit != ExitNotFound {
		t.Errorf("expected exit %d, got %d", ExitNotFound, ce.exit)
	}
}

func TestClassifyError_Config(t *testing.T) {
	err := newConfigError("bad config")
	ce := toCliError(err)

	if ce.code != ErrCodeConfig {
		t.Errorf("expected code %q, got %q", ErrCodeConfig, ce.code)
	}
	if ce.exit != ExitConfig {
		t.Errorf("expected exit %d, got %d", ExitConfig, ce.exit)
	}
}

func TestClassifyError_GenericError(t *testing.T) {
	err := errors.New("something went wrong")
	ce := toCliError(err)

	if ce.code != ErrCodeGeneral {
		t.Errorf("expected code %q, got %q", ErrCodeGeneral, ce.code)
	}
	if ce.exit != ExitErr {
		t.Errorf("expected exit %d, got %d", ExitErr, ce.exit)
	}
	if ce.Error() != "something went wrong" {
		t.Errorf("expected msg %q, got %q", "something went wrong", ce.Error())
	}
}
