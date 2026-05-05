package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteSuccess(t *testing.T) {
	buf := &bytes.Buffer{}
	data := map[string]any{"symbol": "AAPL", "price": "150.00"}
	meta := Metadata{Timestamp: "2026-04-21T10:00:00Z"}

	err := WriteSuccess(buf, data, meta)
	require.NoError(t, err)

	var envelope Envelope
	err = json.Unmarshal(buf.Bytes(), &envelope)
	require.NoError(t, err)

	assert.Equal(t, data, envelope.Data)
	assert.Equal(t, "2026-04-21T10:00:00Z", envelope.Metadata.Timestamp)
	assert.Nil(t, envelope.Errors)
}

func TestWriteSuccessWithZeroMetadata(t *testing.T) {
	buf := &bytes.Buffer{}
	data := map[string]any{"test": "value"}

	err := WriteSuccess(buf, data, Metadata{})
	require.NoError(t, err)

	var envelope Envelope
	err = json.Unmarshal(buf.Bytes(), &envelope)
	require.NoError(t, err)

	assert.Equal(t, data, envelope.Data)
	assert.Nil(t, envelope.Errors)
}

func TestWriteSuccessWithComplexData(t *testing.T) {
	buf := &bytes.Buffer{}
	data := map[string]any{
		"accounts": []map[string]any{
			{"id": "123", "balance": 10000.50},
			{"id": "456", "balance": 25000.75},
		},
	}
	meta := Metadata{Timestamp: "2026-04-21T10:00:00Z"}

	err := WriteSuccess(buf, data, meta)
	require.NoError(t, err)

	var envelope Envelope
	err = json.Unmarshal(buf.Bytes(), &envelope)
	require.NoError(t, err)

	// Verify data structure exists
	assert.NotNil(t, envelope.Data)
	dataMap := envelope.Data.(map[string]any)
	assert.Contains(t, dataMap, "accounts")

	// Verify metadata timestamp
	assert.Equal(t, "2026-04-21T10:00:00Z", envelope.Metadata.Timestamp)
}

func TestWriteErrorAuthRequired(t *testing.T) {
	buf := &bytes.Buffer{}
	err := apperr.NewAuthRequiredError("authentication required", errors.New("no token"))

	writeErr := WriteError(buf, err)
	require.NoError(t, writeErr)

	var errorEnvelope ErrorEnvelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &errorEnvelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, "auth_required", errorEnvelope.Error)
	assert.Equal(t, "authentication required", errorEnvelope.Message)
	assert.Equal(t, 3, errorEnvelope.ExitCode)
}

func TestWriteErrorAuthExpired(t *testing.T) {
	buf := &bytes.Buffer{}
	err := apperr.NewAuthExpiredError("auth expired", errors.New("401 response"))

	writeErr := WriteError(buf, err)
	require.NoError(t, writeErr)

	var errorEnvelope ErrorEnvelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &errorEnvelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, "auth_expired", errorEnvelope.Error)
	assert.Equal(t, "auth expired", errorEnvelope.Message)
	assert.Equal(t, 3, errorEnvelope.ExitCode)
}

func TestWriteErrorAuthCallback(t *testing.T) {
	buf := &bytes.Buffer{}
	err := apperr.NewAuthCallbackError("callback failed", errors.New("invalid state"))

	writeErr := WriteError(buf, err)
	require.NoError(t, writeErr)

	var errorEnvelope ErrorEnvelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &errorEnvelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, "auth_callback", errorEnvelope.Error)
	assert.Equal(t, "callback failed", errorEnvelope.Message)
	assert.Equal(t, 3, errorEnvelope.ExitCode)
}

func TestWriteErrorOrderRejected(t *testing.T) {
	buf := &bytes.Buffer{}
	err := apperr.NewOrderRejectedError("order rejected", errors.New("insufficient funds"))

	writeErr := WriteError(buf, err)
	require.NoError(t, writeErr)

	var errorEnvelope ErrorEnvelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &errorEnvelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, "order_rejected", errorEnvelope.Error)
	assert.Equal(t, "order rejected", errorEnvelope.Message)
	assert.Equal(t, 5, errorEnvelope.ExitCode)
}

func TestWriteErrorSymbolNotFound(t *testing.T) {
	buf := &bytes.Buffer{}
	err := apperr.NewSymbolNotFoundError("symbol not found", errors.New("api returned empty"))

	writeErr := WriteError(buf, err)
	require.NoError(t, writeErr)

	var errorEnvelope ErrorEnvelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &errorEnvelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, "symbol_not_found", errorEnvelope.Error)
	assert.Equal(t, "symbol not found", errorEnvelope.Message)
	assert.Equal(t, 2, errorEnvelope.ExitCode)
}

func TestWriteErrorAccountNotFound(t *testing.T) {
	buf := &bytes.Buffer{}
	err := apperr.NewAccountNotFoundError("account not found", errors.New("invalid account id"))

	writeErr := WriteError(buf, err)
	require.NoError(t, writeErr)

	var errorEnvelope ErrorEnvelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &errorEnvelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, "account_not_found", errorEnvelope.Error)
	assert.Equal(t, "account not found", errorEnvelope.Message)
	assert.Equal(t, 2, errorEnvelope.ExitCode)
}

func TestWriteErrorHTTPError(t *testing.T) {
	buf := &bytes.Buffer{}
	err := apperr.NewHTTPError("http error", 500, "Internal Server Error", errors.New("server error"))

	writeErr := WriteError(buf, err)
	require.NoError(t, writeErr)

	var errorEnvelope ErrorEnvelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &errorEnvelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, "http_error", errorEnvelope.Error)
	assert.Equal(t, "http error", errorEnvelope.Message)
	assert.Equal(t, 4, errorEnvelope.ExitCode)
	assert.Contains(t, errorEnvelope.Got, "500")
	assert.Equal(t, "2xx response", errorEnvelope.Expected)
}

func TestWriteErrorValidationError(t *testing.T) {
	buf := &bytes.Buffer{}
	err := apperr.NewValidationError("validation failed", errors.New("invalid input"))

	writeErr := WriteError(buf, err)
	require.NoError(t, writeErr)

	var errorEnvelope ErrorEnvelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &errorEnvelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, "validation_error", errorEnvelope.Error)
	assert.Equal(t, "validation failed", errorEnvelope.Message)
	assert.Equal(t, 1, errorEnvelope.ExitCode)
}

func TestWriteErrorOrderBuildError(t *testing.T) {
	buf := &bytes.Buffer{}
	err := apperr.NewOrderBuildError("order build failed", errors.New("invalid order params"))

	writeErr := WriteError(buf, err)
	require.NoError(t, writeErr)

	var errorEnvelope ErrorEnvelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &errorEnvelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, "order_build_error", errorEnvelope.Error)
	assert.Equal(t, "order build failed", errorEnvelope.Message)
	assert.Equal(t, 1, errorEnvelope.ExitCode)
}

func TestWriteErrorGenericError(t *testing.T) {
	buf := &bytes.Buffer{}
	err := errors.New("unknown error")

	writeErr := WriteError(buf, err)
	require.NoError(t, writeErr)

	var errorEnvelope ErrorEnvelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &errorEnvelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, "error", errorEnvelope.Error)
	assert.Equal(t, "unknown error", errorEnvelope.Message)
	assert.Equal(t, 1, errorEnvelope.ExitCode)
}

func TestWriteCommandErrorUnknownFlag(t *testing.T) {
	buf := &bytes.Buffer{}
	cmd := &cobra.Command{
		Use:           "test",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
	cmd.SetArgs([]string{"--bogus"})

	executed, err := cmd.ExecuteC()
	require.Error(t, err)

	exitCode, writeErr := WriteCommandError(buf, executed, err)
	require.NoError(t, writeErr)

	var errorEnvelope ErrorEnvelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &errorEnvelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, "unknown_flag", errorEnvelope.Error)
	assert.Equal(t, 12, exitCode)
	assert.Equal(t, 12, errorEnvelope.ExitCode)
	assert.Equal(t, "bogus", errorEnvelope.Flag)
	assert.Equal(t, "test", errorEnvelope.Command)
}

func TestWriteCommandErrorTypedUnknownFlag(t *testing.T) {
	buf := &bytes.Buffer{}
	cmd := &cobra.Command{
		Use:           "test",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
	cmd.SetFlagErrorFunc(apperr.NormalizeFlagError)
	cmd.SetArgs([]string{"--bogus"})

	executed, err := cmd.ExecuteC()
	require.Error(t, err)

	exitCode, writeErr := WriteCommandError(buf, executed, err)
	require.NoError(t, writeErr)

	var errorEnvelope ErrorEnvelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &errorEnvelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, "unknown_flag", errorEnvelope.Error)
	assert.Equal(t, 12, exitCode)
	assert.Equal(t, 12, errorEnvelope.ExitCode)
	assert.Equal(t, "bogus", errorEnvelope.Flag)
	assert.Equal(t, "test", errorEnvelope.Command)
}

func TestWriteCommandErrorTypedInvalidFlagValue(t *testing.T) {
	buf := &bytes.Buffer{}
	cmd := &cobra.Command{
		Use:           "test",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
	cmd.Flags().Int("count", 0, "Number of items")
	cmd.SetFlagErrorFunc(apperr.NormalizeFlagError)
	cmd.SetArgs([]string{"--count", "many"})

	executed, err := cmd.ExecuteC()
	require.Error(t, err)

	exitCode, writeErr := WriteCommandError(buf, executed, err)
	require.NoError(t, writeErr)

	var errorEnvelope ErrorEnvelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &errorEnvelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, "invalid_flag_value", errorEnvelope.Error)
	assert.Equal(t, 11, exitCode)
	assert.Equal(t, 11, errorEnvelope.ExitCode)
	assert.Equal(t, "count", errorEnvelope.Flag)
	assert.Equal(t, "many", errorEnvelope.Got)
	assert.Equal(t, "test", errorEnvelope.Command)
}

func TestWriteErrorWithDetails(t *testing.T) {
	buf := &bytes.Buffer{}
	err := apperr.NewSchwabError("custom error", errors.New("cause"), apperr.WithDetails("additional context"))

	writeErr := WriteError(buf, err)
	require.NoError(t, writeErr)

	var errorEnvelope ErrorEnvelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &errorEnvelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, "additional context", errorEnvelope.Hint)
}

func TestWritePartial(t *testing.T) {
	buf := &bytes.Buffer{}
	data := map[string]any{"partial": "data"}
	errs := []string{"error 1", "error 2"}
	meta := Metadata{Timestamp: "2026-04-21T10:00:00Z"}

	writeErr := WritePartial(buf, data, errs, meta)
	require.NoError(t, writeErr)

	var envelope Envelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &envelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, data, envelope.Data)
	assert.Equal(t, errs, envelope.Errors)
	assert.Equal(t, "2026-04-21T10:00:00Z", envelope.Metadata.Timestamp)
}

func TestWritePartialWithEmptyErrors(t *testing.T) {
	buf := &bytes.Buffer{}
	data := map[string]any{"test": "data"}
	meta := Metadata{Timestamp: "2026-04-21T10:00:00Z"}

	writeErr := WritePartial(buf, data, []string{}, meta)
	require.NoError(t, writeErr)

	var envelope Envelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &envelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, data, envelope.Data)
	assert.Nil(t, envelope.Errors)
	assert.Equal(t, "2026-04-21T10:00:00Z", envelope.Metadata.Timestamp)
}

func TestWritePartialWithNilData(t *testing.T) {
	buf := &bytes.Buffer{}
	errs := []string{"error 1"}
	meta := Metadata{Timestamp: "2026-04-21T10:00:00Z"}

	writeErr := WritePartial(buf, nil, errs, meta)
	require.NoError(t, writeErr)

	var envelope Envelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &envelope)
	require.NoError(t, unmarshalErr)

	assert.Nil(t, envelope.Data)
	assert.Equal(t, errs, envelope.Errors)
	assert.Equal(t, "2026-04-21T10:00:00Z", envelope.Metadata.Timestamp)
}

func TestJSONEscapeHTMLDisabled(t *testing.T) {
	buf := &bytes.Buffer{}
	data := map[string]string{"url": "https://example.com?foo=bar&baz=qux"}

	err := WriteSuccess(buf, data, Metadata{})
	require.NoError(t, err)

	// Check that & is not escaped to \u0026
	assert.NotContains(t, buf.String(), "\\u0026")
	assert.Contains(t, buf.String(), "&")
}

func TestNewMetadata(t *testing.T) {
	meta := NewMetadata()

	// Verify timestamp is a valid RFC3339 string
	assert.NotEmpty(t, meta.Timestamp)

	_, err := time.Parse(time.RFC3339, meta.Timestamp)
	assert.NoError(t, err)
}

func TestNewMetadataIsUTC(t *testing.T) {
	meta := NewMetadata()

	// RFC3339 with Z suffix indicates UTC
	assert.Contains(t, meta.Timestamp, "Z")
}

func TestEnvelopeJSONStructure(t *testing.T) {
	buf := &bytes.Buffer{}
	data := map[string]any{"key": "value"}
	meta := Metadata{Timestamp: "2026-04-21T10:00:00Z"}

	err := WriteSuccess(buf, data, meta)
	require.NoError(t, err)

	// Verify JSON structure
	var raw map[string]any
	err = json.Unmarshal(buf.Bytes(), &raw)
	require.NoError(t, err)

	assert.Contains(t, raw, "data")
	assert.Contains(t, raw, "metadata")
}

func TestErrorEnvelopeJSONStructure(t *testing.T) {
	buf := &bytes.Buffer{}
	err := apperr.NewAuthRequiredError("auth required", errors.New("no token"))

	writeErr := WriteError(buf, err)
	require.NoError(t, writeErr)

	// Verify JSON structure
	var raw map[string]any
	unmarshalErr := json.Unmarshal(buf.Bytes(), &raw)
	require.NoError(t, unmarshalErr)

	assert.Contains(t, raw, "error")
	assert.Contains(t, raw, "exit_code")
	assert.Contains(t, raw, "message")
}

func TestWriteSuccessWithEmptyMetadata(t *testing.T) {
	buf := &bytes.Buffer{}
	data := map[string]any{"test": "data"}

	err := WriteSuccess(buf, data, Metadata{})
	require.NoError(t, err)

	var envelope Envelope
	err = json.Unmarshal(buf.Bytes(), &envelope)
	require.NoError(t, err)

	assert.Equal(t, data, envelope.Data)
	assert.Empty(t, envelope.Metadata.Timestamp)
}

func TestWritePartialMultipleErrors(t *testing.T) {
	buf := &bytes.Buffer{}
	data := map[string]any{"partial": "result"}
	errs := []string{"error 1", "error 2", "error 3"}
	meta := Metadata{Timestamp: "2026-04-21T10:00:00Z"}

	writeErr := WritePartial(buf, data, errs, meta)
	require.NoError(t, writeErr)

	var envelope Envelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &envelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, 3, len(envelope.Errors))
	assert.Equal(t, errs, envelope.Errors)
}

// TestMetadataAccountContextFields verifies the 4 new account context fields
// are properly serialized when populated and omitted when empty.
func TestMetadataAccountContextFields(t *testing.T) {
	tests := []struct {
		name     string
		meta     Metadata
		wantKeys []string
		notKeys  []string
	}{
		{
			name: "all account context fields populated",
			meta: Metadata{
				Timestamp:           "2026-04-21T10:00:00Z",
				Account:             "abc123",
				AccountNickName:     "My Roth IRA",
				AccountType:         "ROTH_IRA",
				AccountSource:       "explicit",
				AccountDisplayLabel: "Roth IRA - My Roth IRA",
			},
			wantKeys: []string{"timestamp", "account", "accountNickName", "accountType", "accountSource", "accountDisplayLabel"},
			notKeys:  []string{},
		},
		{
			name: "only timestamp and account, new fields empty",
			meta: Metadata{
				Timestamp: "2026-04-21T10:00:00Z",
				Account:   "abc123",
			},
			wantKeys: []string{"timestamp", "account"},
			notKeys:  []string{"accountNickName", "accountType", "accountSource", "accountDisplayLabel"},
		},
		{
			name: "only new account context fields populated",
			meta: Metadata{
				Timestamp:           "2026-04-21T10:00:00Z",
				AccountNickName:     "Trading Account",
				AccountType:         "INDIVIDUAL",
				AccountSource:       "default",
				AccountDisplayLabel: "Individual - Trading Account",
			},
			wantKeys: []string{"timestamp", "accountNickName", "accountType", "accountSource", "accountDisplayLabel"},
			notKeys:  []string{"account"},
		},
		{
			name: "partial account context fields",
			meta: Metadata{
				Timestamp:       "2026-04-21T10:00:00Z",
				AccountNickName: "Brokerage",
				AccountType:     "INDIVIDUAL",
			},
			wantKeys: []string{"timestamp", "accountNickName", "accountType"},
			notKeys:  []string{"account", "accountSource", "accountDisplayLabel"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			err := WriteSuccess(buf, map[string]any{"test": "data"}, tt.meta)
			require.NoError(t, err)

			var envelope Envelope
			err = json.Unmarshal(buf.Bytes(), &envelope)
			require.NoError(t, err)

			// Verify wanted keys are present
			for _, key := range tt.wantKeys {
				switch key {
				case "timestamp":
					assert.Equal(t, tt.meta.Timestamp, envelope.Metadata.Timestamp)
				case "account":
					assert.Equal(t, tt.meta.Account, envelope.Metadata.Account)
				case "accountNickName":
					assert.Equal(t, tt.meta.AccountNickName, envelope.Metadata.AccountNickName)
				case "accountType":
					assert.Equal(t, tt.meta.AccountType, envelope.Metadata.AccountType)
				case "accountSource":
					assert.Equal(t, tt.meta.AccountSource, envelope.Metadata.AccountSource)
				case "accountDisplayLabel":
					assert.Equal(t, tt.meta.AccountDisplayLabel, envelope.Metadata.AccountDisplayLabel)
				}
			}

			// Verify unwanted keys are omitted from JSON (omitempty behavior)
			rawJSON := buf.String()
			for _, key := range tt.notKeys {
				assert.NotContains(t, rawJSON, `"`+key+`"`, "field %s should be omitted from JSON when empty", key)
			}
		})
	}
}

// TestMetadataAccountContextFieldsOmitempty verifies that empty account context
// fields are properly omitted from JSON output due to omitempty tag.
func TestMetadataAccountContextFieldsOmitempty(t *testing.T) {
	buf := &bytes.Buffer{}
	meta := Metadata{
		Timestamp: "2026-04-21T10:00:00Z",
		Account:   "abc123",
		// All new fields left empty
	}

	err := WriteSuccess(buf, map[string]any{"test": "data"}, meta)
	require.NoError(t, err)

	rawJSON := buf.String()

	// Verify empty fields are not in JSON output
	assert.NotContains(t, rawJSON, "accountNickName")
	assert.NotContains(t, rawJSON, "accountType")
	assert.NotContains(t, rawJSON, "accountSource")
	assert.NotContains(t, rawJSON, "accountDisplayLabel")

	// Verify populated fields are present
	assert.Contains(t, rawJSON, "timestamp")
	assert.Contains(t, rawJSON, "account")
}

// TestMetadataAccountContextFieldsJSON verifies the exact JSON structure
// with all new fields populated.
func TestMetadataAccountContextFieldsJSON(t *testing.T) {
	buf := &bytes.Buffer{}
	meta := Metadata{
		Timestamp:           "2026-04-21T10:00:00Z",
		Account:             "abc123",
		AccountNickName:     "My Roth IRA",
		AccountType:         "ROTH_IRA",
		AccountSource:       "preview",
		AccountDisplayLabel: "Roth IRA - My Roth IRA",
	}

	err := WriteSuccess(buf, map[string]any{"test": "data"}, meta)
	require.NoError(t, err)

	var envelope Envelope
	err = json.Unmarshal(buf.Bytes(), &envelope)
	require.NoError(t, err)

	// Verify all fields are correctly populated
	assert.Equal(t, "2026-04-21T10:00:00Z", envelope.Metadata.Timestamp)
	assert.Equal(t, "abc123", envelope.Metadata.Account)
	assert.Equal(t, "My Roth IRA", envelope.Metadata.AccountNickName)
	assert.Equal(t, "ROTH_IRA", envelope.Metadata.AccountType)
	assert.Equal(t, "preview", envelope.Metadata.AccountSource)
	assert.Equal(t, "Roth IRA - My Roth IRA", envelope.Metadata.AccountDisplayLabel)
}

// TestMetadataBackwardCompatibility verifies that existing tests still pass
// and that the new fields don't break existing functionality.
func TestMetadataBackwardCompatibility(t *testing.T) {
	buf := &bytes.Buffer{}
	data := map[string]any{"symbol": "AAPL", "price": "150.00"}
	meta := Metadata{
		Timestamp: "2026-04-21T10:00:00Z",
		Account:   "xyz789",
		Requested: 1,
		Returned:  1,
	}

	err := WriteSuccess(buf, data, meta)
	require.NoError(t, err)

	var envelope Envelope
	err = json.Unmarshal(buf.Bytes(), &envelope)
	require.NoError(t, err)

	// Verify existing fields still work
	assert.Equal(t, data, envelope.Data)
	assert.Equal(t, "2026-04-21T10:00:00Z", envelope.Metadata.Timestamp)
	assert.Equal(t, "xyz789", envelope.Metadata.Account)
	assert.Equal(t, 1, envelope.Metadata.Requested)
	assert.Equal(t, 1, envelope.Metadata.Returned)

	// Verify new fields are empty and omitted
	assert.Empty(t, envelope.Metadata.AccountNickName)
	assert.Empty(t, envelope.Metadata.AccountType)
	assert.Empty(t, envelope.Metadata.AccountSource)
	assert.Empty(t, envelope.Metadata.AccountDisplayLabel)
}
