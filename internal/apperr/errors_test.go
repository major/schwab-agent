package apperr

import (
	"errors"
	"fmt"
	"testing"

	structclierrors "github.com/leodido/structcli/errors"
)

// TestExitCodeForAuthRequiredError verifies AuthRequiredError maps to exit code 3.
func TestExitCodeForAuthRequiredError(t *testing.T) {
	err := NewAuthRequiredError("auth required", nil)
	code := ExitCodeFor(err)
	if code != 3 {
		t.Errorf("ExitCodeFor(AuthRequiredError) = %d, want 3", code)
	}
}

// TestExitCodeForAuthExpiredError verifies AuthExpiredError maps to exit code 3.
func TestExitCodeForAuthExpiredError(t *testing.T) {
	err := NewAuthExpiredError("auth expired", nil)
	code := ExitCodeFor(err)
	if code != 3 {
		t.Errorf("ExitCodeFor(AuthExpiredError) = %d, want 3", code)
	}
}

// TestExitCodeForAuthCallbackError verifies AuthCallbackError maps to exit code 3.
func TestExitCodeForAuthCallbackError(t *testing.T) {
	err := NewAuthCallbackError("auth callback failed", nil)
	code := ExitCodeFor(err)
	if code != 3 {
		t.Errorf("ExitCodeFor(AuthCallbackError) = %d, want 3", code)
	}
}

// TestExitCodeForOrderRejectedError verifies OrderRejectedError maps to exit code 5.
func TestExitCodeForOrderRejectedError(t *testing.T) {
	err := NewOrderRejectedError("order rejected", nil)
	code := ExitCodeFor(err)
	if code != 5 {
		t.Errorf("ExitCodeFor(OrderRejectedError) = %d, want 5", code)
	}
}

// TestExitCodeForSymbolNotFoundError verifies SymbolNotFoundError maps to exit code 2.
func TestExitCodeForSymbolNotFoundError(t *testing.T) {
	err := NewSymbolNotFoundError("symbol not found", nil)
	code := ExitCodeFor(err)
	if code != 2 {
		t.Errorf("ExitCodeFor(SymbolNotFoundError) = %d, want 2", code)
	}
}

// TestExitCodeForAccountNotFoundError verifies AccountNotFoundError maps to exit code 2.
func TestExitCodeForAccountNotFoundError(t *testing.T) {
	err := NewAccountNotFoundError("account not found", nil)
	code := ExitCodeFor(err)
	if code != 2 {
		t.Errorf("ExitCodeFor(AccountNotFoundError) = %d, want 2", code)
	}
}

// TestExitCodeForHTTPError verifies HTTPError maps to exit code 4.
func TestExitCodeForHTTPError(t *testing.T) {
	err := NewHTTPError("http error", 500, "Internal Server Error", nil)
	code := ExitCodeFor(err)
	if code != 4 {
		t.Errorf("ExitCodeFor(HTTPError) = %d, want 4", code)
	}
}

// TestExitCodeForValidationError verifies ValidationError maps to exit code 1.
func TestExitCodeForValidationError(t *testing.T) {
	err := NewValidationError("validation failed", nil)
	code := ExitCodeFor(err)
	if code != 1 {
		t.Errorf("ExitCodeFor(ValidationError) = %d, want 1", code)
	}
}

// TestExitCodeForOrderBuildError verifies OrderBuildError maps to exit code 1.
func TestExitCodeForOrderBuildError(t *testing.T) {
	err := NewOrderBuildError("order build failed", nil)
	code := ExitCodeFor(err)
	if code != 1 {
		t.Errorf("ExitCodeFor(OrderBuildError) = %d, want 1", code)
	}
}

// TestExitCodeForSchwabError verifies SchwabError maps to exit code 1.
func TestExitCodeForSchwabError(t *testing.T) {
	err := NewSchwabError("generic error", nil)
	code := ExitCodeFor(err)
	if code != 1 {
		t.Errorf("ExitCodeFor(SchwabError) = %d, want 1", code)
	}
}

// TestExitCodeForNilError verifies nil error maps to exit code 0.
func TestExitCodeForNilError(t *testing.T) {
	code := ExitCodeFor(nil)
	if code != 0 {
		t.Errorf("ExitCodeFor(nil) = %d, want 0", code)
	}
}

// TestExitCodeForStandardError verifies standard errors.New() maps to exit code 1.
func TestExitCodeForStandardError(t *testing.T) {
	err := errors.New("standard error")
	code := ExitCodeFor(err)
	if code != 1 {
		t.Errorf("ExitCodeFor(standard error) = %d, want 1", code)
	}
}

// TestErrorCodeForAuthRequiredError verifies ErrorCode returns correct string.
func TestErrorCodeForAuthRequiredError(t *testing.T) {
	err := NewAuthRequiredError("auth required", nil)
	code := ErrorCode(err)
	if code != "AUTH_REQUIRED" {
		t.Errorf("ErrorCode(AuthRequiredError) = %q, want %q", code, "AUTH_REQUIRED")
	}
}

// TestErrorCodeForAuthExpiredError verifies ErrorCode returns correct string.
func TestErrorCodeForAuthExpiredError(t *testing.T) {
	err := NewAuthExpiredError("auth expired", nil)
	code := ErrorCode(err)
	if code != "AUTH_EXPIRED" {
		t.Errorf("ErrorCode(AuthExpiredError) = %q, want %q", code, "AUTH_EXPIRED")
	}
}

// TestErrorCodeForAuthCallbackError verifies ErrorCode returns correct string.
func TestErrorCodeForAuthCallbackError(t *testing.T) {
	err := NewAuthCallbackError("auth callback failed", nil)
	code := ErrorCode(err)
	if code != "AUTH_CALLBACK" {
		t.Errorf("ErrorCode(AuthCallbackError) = %q, want %q", code, "AUTH_CALLBACK")
	}
}

// TestErrorCodeForOrderRejectedError verifies ErrorCode returns correct string.
func TestErrorCodeForOrderRejectedError(t *testing.T) {
	err := NewOrderRejectedError("order rejected", nil)
	code := ErrorCode(err)
	if code != "ORDER_REJECTED" {
		t.Errorf("ErrorCode(OrderRejectedError) = %q, want %q", code, "ORDER_REJECTED")
	}
}

// TestErrorCodeForSymbolNotFoundError verifies ErrorCode returns correct string.
func TestErrorCodeForSymbolNotFoundError(t *testing.T) {
	err := NewSymbolNotFoundError("symbol not found", nil)
	code := ErrorCode(err)
	if code != "SYMBOL_NOT_FOUND" {
		t.Errorf("ErrorCode(SymbolNotFoundError) = %q, want %q", code, "SYMBOL_NOT_FOUND")
	}
}

// TestErrorCodeForAccountNotFoundError verifies ErrorCode returns correct string.
func TestErrorCodeForAccountNotFoundError(t *testing.T) {
	err := NewAccountNotFoundError("account not found", nil)
	code := ErrorCode(err)
	if code != "ACCOUNT_NOT_FOUND" {
		t.Errorf("ErrorCode(AccountNotFoundError) = %q, want %q", code, "ACCOUNT_NOT_FOUND")
	}
}

// TestErrorCodeForHTTPError verifies ErrorCode returns correct string.
func TestErrorCodeForHTTPError(t *testing.T) {
	err := NewHTTPError("http error", 500, "Internal Server Error", nil)
	code := ErrorCode(err)
	if code != "HTTP_ERROR" {
		t.Errorf("ErrorCode(HTTPError) = %q, want %q", code, "HTTP_ERROR")
	}
}

// TestErrorCodeForValidationError verifies ErrorCode returns correct string.
func TestErrorCodeForValidationError(t *testing.T) {
	err := NewValidationError("validation failed", nil)
	code := ErrorCode(err)
	if code != "VALIDATION_ERROR" {
		t.Errorf("ErrorCode(ValidationError) = %q, want %q", code, "VALIDATION_ERROR")
	}
}

// TestErrorCodeForOrderBuildError verifies ErrorCode returns correct string.
func TestErrorCodeForOrderBuildError(t *testing.T) {
	err := NewOrderBuildError("order build failed", nil)
	code := ErrorCode(err)
	if code != "ORDER_BUILD_ERROR" {
		t.Errorf("ErrorCode(OrderBuildError) = %q, want %q", code, "ORDER_BUILD_ERROR")
	}
}

// TestErrorCodeForStructcliValidationError verifies structcli's ValidationError
// (produced by the Validatable interface) maps to VALIDATION_ERROR, same as
// our internal ValidationError.
func TestErrorCodeForStructcliValidationError(t *testing.T) {
	err := &structclierrors.ValidationError{
		ContextName: "test-cmd",
		Errors:      []error{fmt.Errorf("field is invalid")},
	}
	code := ErrorCode(err)
	if code != "VALIDATION_ERROR" {
		t.Errorf("ErrorCode(structcli ValidationError) = %q, want %q", code, "VALIDATION_ERROR")
	}
}

// TestExitCodeForStructcliValidationError verifies structcli's ValidationError
// maps to exit code 1 (default for non-exitCoder errors).
func TestExitCodeForStructcliValidationError(t *testing.T) {
	err := &structclierrors.ValidationError{
		ContextName: "test-cmd",
		Errors:      []error{fmt.Errorf("field is invalid")},
	}
	code := ExitCodeFor(err)
	if code != 1 {
		t.Errorf("ExitCodeFor(structcli ValidationError) = %d, want 1", code)
	}
}

// TestErrorCodeForSchwabError verifies ErrorCode returns correct string.
func TestErrorCodeForSchwabError(t *testing.T) {
	err := NewSchwabError("generic error", nil)
	code := ErrorCode(err)
	if code != "UNKNOWN" {
		t.Errorf("ErrorCode(SchwabError) = %q, want %q", code, "UNKNOWN")
	}
}

// TestErrorCodeForStandardError verifies ErrorCode returns UNKNOWN for standard errors.
func TestErrorCodeForStandardError(t *testing.T) {
	err := errors.New("standard error")
	code := ErrorCode(err)
	if code != "UNKNOWN" {
		t.Errorf("ErrorCode(standard error) = %q, want %q", code, "UNKNOWN")
	}
}

// TestErrorMessagePreservation verifies error messages are preserved.
func TestErrorMessagePreservation(t *testing.T) {
	message := "test error message"
	err := NewSchwabError(message, nil)
	if err.Error() != message {
		t.Errorf("Error() = %q, want %q", err.Error(), message)
	}
}

// TestErrorChainPreservation verifies error chains are preserved through wrapping.
func TestErrorChainPreservation(t *testing.T) {
	underlying := errors.New("root cause")
	err := NewAuthRequiredError("auth required", underlying)

	if !errors.Is(err, underlying) {
		t.Error("error chain not preserved: errors.Is(err, underlying) returned false")
	}

	if !errors.Is(err.Unwrap(), underlying) {
		t.Error("Unwrap() did not return the underlying error")
	}
}

// TestErrorChainTraversal verifies errors.AsType() can traverse the error chain.
func TestErrorChainTraversal(t *testing.T) {
	underlying := errors.New("root cause")
	err := NewAuthRequiredError("auth required", underlying)

	authErr, ok := errors.AsType[*AuthRequiredError](err)
	if !ok {
		t.Error("errors.AsType() failed to find AuthRequiredError in chain")
	}

	// Note: errors.AsType() does not automatically traverse to parent types
	// without explicit As() method implementation. This test verifies
	// that the specific error type can be found.
	if authErr.Message != "auth required" {
		t.Errorf("AuthRequiredError message = %q, want %q", authErr.Message, "auth required")
	}
}

// TestHTTPErrorAttributes verifies HTTPError stores status code and body.
func TestHTTPErrorAttributes(t *testing.T) {
	statusCode := 500
	body := "Internal Server Error"
	err := NewHTTPError("http error", statusCode, body, nil)
	if err.StatusCode != statusCode {
		t.Errorf("HTTPError.StatusCode = %d, want %d", err.StatusCode, statusCode)
	}
	if err.Body != body {
		t.Errorf("HTTPError.Body = %q, want %q", err.Body, body)
	}
}

// TestDetailsFieldPreservation verifies Details is set through constructor option.
func TestDetailsFieldPreservation(t *testing.T) {
	details := "Try checking your credentials"
	err := NewAuthRequiredError("auth required", nil, WithDetails(details))
	if err.Details() != details {
		t.Errorf("Details() = %q, want %q", err.Details(), details)
	}
}

// TestWrappedAuthRequiredError verifies wrapped AuthRequiredError maps correctly.
func TestWrappedAuthRequiredError(t *testing.T) {
	underlying := errors.New("invalid token")
	err := NewAuthRequiredError("auth required", underlying)
	code := ExitCodeFor(err)
	if code != 3 {
		t.Errorf("ExitCodeFor(wrapped AuthRequiredError) = %d, want 3", code)
	}
}

// TestWrappedOrderRejectedError verifies wrapped OrderRejectedError maps correctly.
func TestWrappedOrderRejectedError(t *testing.T) {
	underlying := errors.New("insufficient funds")
	err := NewOrderRejectedError("order rejected", underlying)
	code := ExitCodeFor(err)
	if code != 5 {
		t.Errorf("ExitCodeFor(wrapped OrderRejectedError) = %d, want 5", code)
	}
}

// TestWrappedSymbolNotFoundError verifies wrapped SymbolNotFoundError maps correctly.
func TestWrappedSymbolNotFoundError(t *testing.T) {
	underlying := errors.New("api returned empty data")
	err := NewSymbolNotFoundError("symbol not found", underlying)
	code := ExitCodeFor(err)
	if code != 2 {
		t.Errorf("ExitCodeFor(wrapped SymbolNotFoundError) = %d, want 2", code)
	}
}

// TestWrappedAccountNotFoundError verifies wrapped AccountNotFoundError maps correctly.
func TestWrappedAccountNotFoundError(t *testing.T) {
	underlying := errors.New("account lookup failed")
	err := NewAccountNotFoundError("account not found", underlying)
	code := ExitCodeFor(err)
	if code != 2 {
		t.Errorf("ExitCodeFor(wrapped AccountNotFoundError) = %d, want 2", code)
	}
}

// TestWrappedHTTPError verifies wrapped HTTPError maps correctly.
func TestWrappedHTTPError(t *testing.T) {
	underlying := errors.New("connection failed")
	err := NewHTTPError("http error", 502, "Bad Gateway", underlying)
	code := ExitCodeFor(err)
	if code != 4 {
		t.Errorf("ExitCodeFor(wrapped HTTPError) = %d, want 4", code)
	}
}

// TestWrappedValidationError verifies wrapped ValidationError maps correctly.
func TestWrappedValidationError(t *testing.T) {
	underlying := errors.New("symbol is empty")
	err := NewValidationError("validation failed", underlying)
	code := ExitCodeFor(err)
	if code != 1 {
		t.Errorf("ExitCodeFor(wrapped ValidationError) = %d, want 1", code)
	}
}

// TestWrappedOrderBuildError verifies wrapped OrderBuildError maps correctly.
func TestWrappedOrderBuildError(t *testing.T) {
	underlying := errors.New("invalid order parameters")
	err := NewOrderBuildError("order build failed", underlying)
	code := ExitCodeFor(err)
	if code != 1 {
		t.Errorf("ExitCodeFor(wrapped OrderBuildError) = %d, want 1", code)
	}
}

// TestWithDetails_AllConstructors verifies the WithDetails option sets the
// details field on every error constructor. The existing TestDetailsFieldPreservation
// covers AuthRequiredError; this table-driven test covers the remaining types.
func TestWithDetails_AllConstructors(t *testing.T) {
	const hint = "check your settings"

	tests := []struct {
		name    string
		details string
	}{
		{"AuthExpired", NewAuthExpiredError("expired", nil, WithDetails(hint)).Details()},
		{"AuthCallback", NewAuthCallbackError("callback", nil, WithDetails(hint)).Details()},
		{"OrderRejected", NewOrderRejectedError("rejected", nil, WithDetails(hint)).Details()},
		{"SymbolNotFound", NewSymbolNotFoundError("not found", nil, WithDetails(hint)).Details()},
		{"AccountNotFound", NewAccountNotFoundError("not found", nil, WithDetails(hint)).Details()},
		{"HTTPError", NewHTTPError("http", 500, "body", nil, WithDetails(hint)).Details()},
		{"Validation", NewValidationError("invalid", nil, WithDetails(hint)).Details()},
		{"OrderBuild", NewOrderBuildError("build", nil, WithDetails(hint)).Details()},
		{"SchwabError", NewSchwabError("generic", nil, WithDetails(hint)).Details()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.details != hint {
				t.Errorf("Details() = %q, want %q", tt.details, hint)
			}
		})
	}
}
