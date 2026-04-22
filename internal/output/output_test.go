package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"

	schwabErrors "github.com/major/schwab-agent/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteSuccess(t *testing.T) {
	buf := &bytes.Buffer{}
	data := map[string]any{"symbol": "AAPL", "price": "150.00"}
	metadata := map[string]any{"timestamp": "2026-04-21T10:00:00Z"}

	err := WriteSuccess(buf, data, metadata)
	require.NoError(t, err)

	var envelope Envelope
	err = json.Unmarshal(buf.Bytes(), &envelope)
	require.NoError(t, err)

	assert.Equal(t, data, envelope.Data)
	assert.Equal(t, metadata, envelope.Metadata)
	assert.Nil(t, envelope.Errors)
}

func TestWriteSuccessWithNilMetadata(t *testing.T) {
	buf := &bytes.Buffer{}
	data := map[string]any{"test": "value"}

	err := WriteSuccess(buf, data, nil)
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
	metadata := map[string]any{"count": 2}

	err := WriteSuccess(buf, data, metadata)
	require.NoError(t, err)

	var envelope Envelope
	err = json.Unmarshal(buf.Bytes(), &envelope)
	require.NoError(t, err)

	// Verify data structure exists
	assert.NotNil(t, envelope.Data)
	dataMap := envelope.Data.(map[string]any)
	assert.Contains(t, dataMap, "accounts")

	// Verify metadata exists
	assert.NotNil(t, envelope.Metadata)
}

func TestWriteErrorAuthRequired(t *testing.T) {
	buf := &bytes.Buffer{}
	err := schwabErrors.NewAuthRequiredError("authentication required", errors.New("no token"))

	writeErr := WriteError(buf, err)
	require.NoError(t, writeErr)

	var errorEnvelope ErrorEnvelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &errorEnvelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, "AUTH_REQUIRED", errorEnvelope.Error.Code)
	assert.Equal(t, "authentication required", errorEnvelope.Error.Message)
}

func TestWriteErrorAuthExpired(t *testing.T) {
	buf := &bytes.Buffer{}
	err := schwabErrors.NewAuthExpiredError("auth expired", errors.New("401 response"))

	writeErr := WriteError(buf, err)
	require.NoError(t, writeErr)

	var errorEnvelope ErrorEnvelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &errorEnvelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, "AUTH_EXPIRED", errorEnvelope.Error.Code)
	assert.Equal(t, "auth expired", errorEnvelope.Error.Message)
}

func TestWriteErrorAuthCallback(t *testing.T) {
	buf := &bytes.Buffer{}
	err := schwabErrors.NewAuthCallbackError("callback failed", errors.New("invalid state"))

	writeErr := WriteError(buf, err)
	require.NoError(t, writeErr)

	var errorEnvelope ErrorEnvelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &errorEnvelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, "AUTH_CALLBACK", errorEnvelope.Error.Code)
	assert.Equal(t, "callback failed", errorEnvelope.Error.Message)
}

func TestWriteErrorOrderRejected(t *testing.T) {
	buf := &bytes.Buffer{}
	err := schwabErrors.NewOrderRejectedError("order rejected", errors.New("insufficient funds"))

	writeErr := WriteError(buf, err)
	require.NoError(t, writeErr)

	var errorEnvelope ErrorEnvelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &errorEnvelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, "ORDER_REJECTED", errorEnvelope.Error.Code)
	assert.Equal(t, "order rejected", errorEnvelope.Error.Message)
}

func TestWriteErrorSymbolNotFound(t *testing.T) {
	buf := &bytes.Buffer{}
	err := schwabErrors.NewSymbolNotFoundError("symbol not found", errors.New("api returned empty"))

	writeErr := WriteError(buf, err)
	require.NoError(t, writeErr)

	var errorEnvelope ErrorEnvelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &errorEnvelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, "SYMBOL_NOT_FOUND", errorEnvelope.Error.Code)
	assert.Equal(t, "symbol not found", errorEnvelope.Error.Message)
}

func TestWriteErrorAccountNotFound(t *testing.T) {
	buf := &bytes.Buffer{}
	err := schwabErrors.NewAccountNotFoundError("account not found", errors.New("invalid account id"))

	writeErr := WriteError(buf, err)
	require.NoError(t, writeErr)

	var errorEnvelope ErrorEnvelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &errorEnvelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, "ACCOUNT_NOT_FOUND", errorEnvelope.Error.Code)
	assert.Equal(t, "account not found", errorEnvelope.Error.Message)
}

func TestWriteErrorHTTPError(t *testing.T) {
	buf := &bytes.Buffer{}
	err := schwabErrors.NewHTTPError("http error", 500, "Internal Server Error", errors.New("server error"))

	writeErr := WriteError(buf, err)
	require.NoError(t, writeErr)

	var errorEnvelope ErrorEnvelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &errorEnvelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, "HTTP_ERROR", errorEnvelope.Error.Code)
	assert.Equal(t, "http error", errorEnvelope.Error.Message)
	assert.Contains(t, errorEnvelope.Error.Details, "500")
}

func TestWriteErrorValidationError(t *testing.T) {
	buf := &bytes.Buffer{}
	err := schwabErrors.NewValidationError("validation failed", errors.New("invalid input"))

	writeErr := WriteError(buf, err)
	require.NoError(t, writeErr)

	var errorEnvelope ErrorEnvelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &errorEnvelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, "VALIDATION_ERROR", errorEnvelope.Error.Code)
	assert.Equal(t, "validation failed", errorEnvelope.Error.Message)
}

func TestWriteErrorOrderBuildError(t *testing.T) {
	buf := &bytes.Buffer{}
	err := schwabErrors.NewOrderBuildError("order build failed", errors.New("invalid order params"))

	writeErr := WriteError(buf, err)
	require.NoError(t, writeErr)

	var errorEnvelope ErrorEnvelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &errorEnvelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, "ORDER_BUILD_ERROR", errorEnvelope.Error.Code)
	assert.Equal(t, "order build failed", errorEnvelope.Error.Message)
}

func TestWriteErrorGenericError(t *testing.T) {
	buf := &bytes.Buffer{}
	err := errors.New("unknown error")

	writeErr := WriteError(buf, err)
	require.NoError(t, writeErr)

	var errorEnvelope ErrorEnvelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &errorEnvelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, "UNKNOWN", errorEnvelope.Error.Code)
	assert.Equal(t, "unknown error", errorEnvelope.Error.Message)
}

func TestWriteErrorWithDetails(t *testing.T) {
	buf := &bytes.Buffer{}
	err := schwabErrors.NewSchwabError("custom error", errors.New("cause"))
	err.Details = "additional context"

	writeErr := WriteError(buf, err)
	require.NoError(t, writeErr)

	var errorEnvelope ErrorEnvelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &errorEnvelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, "additional context", errorEnvelope.Error.Details)
}

func TestWritePartial(t *testing.T) {
	buf := &bytes.Buffer{}
	data := map[string]any{"partial": "data"}
	errs := []string{"error 1", "error 2"}
	metadata := map[string]any{"timestamp": "2026-04-21T10:00:00Z"}

	writeErr := WritePartial(buf, data, errs, metadata)
	require.NoError(t, writeErr)

	var envelope Envelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &envelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, data, envelope.Data)
	assert.Equal(t, errs, envelope.Errors)
	assert.Equal(t, metadata, envelope.Metadata)
}

func TestWritePartialWithEmptyErrors(t *testing.T) {
	buf := &bytes.Buffer{}
	data := map[string]any{"test": "data"}
	metadata := map[string]any{"timestamp": "2026-04-21T10:00:00Z"}

	writeErr := WritePartial(buf, data, []string{}, metadata)
	require.NoError(t, writeErr)

	var envelope Envelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &envelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, data, envelope.Data)
	assert.Nil(t, envelope.Errors)
	assert.Equal(t, metadata, envelope.Metadata)
}

func TestWritePartialWithNilData(t *testing.T) {
	buf := &bytes.Buffer{}
	errs := []string{"error 1"}
	metadata := map[string]any{"timestamp": "2026-04-21T10:00:00Z"}

	writeErr := WritePartial(buf, nil, errs, metadata)
	require.NoError(t, writeErr)

	var envelope Envelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &envelope)
	require.NoError(t, unmarshalErr)

	assert.Nil(t, envelope.Data)
	assert.Equal(t, errs, envelope.Errors)
	assert.Equal(t, metadata, envelope.Metadata)
}

func TestJSONEscapeHTMLDisabled(t *testing.T) {
	buf := &bytes.Buffer{}
	data := map[string]string{"url": "https://example.com?foo=bar&baz=qux"}
	metadata := map[string]any{}

	err := WriteSuccess(buf, data, metadata)
	require.NoError(t, err)

	// Check that & is not escaped to \u0026
	assert.NotContains(t, buf.String(), "\\u0026")
	assert.Contains(t, buf.String(), "&")
}

func TestTimestampMeta(t *testing.T) {
	meta := TimestampMeta()

	assert.NotNil(t, meta["timestamp"])

	// Verify timestamp is a valid RFC3339 string
	timestamp, ok := meta["timestamp"].(string)
	assert.True(t, ok)
	assert.NotEmpty(t, timestamp)

	// Verify it can be parsed as RFC3339
	_, err := time.Parse(time.RFC3339, timestamp)
	assert.NoError(t, err)
}

func TestTimestampMetaIsUTC(t *testing.T) {
	meta := TimestampMeta()
	timestamp := meta["timestamp"].(string)

	// RFC3339 with Z suffix indicates UTC
	assert.Contains(t, timestamp, "Z")
}

func TestEnvelopeJSONStructure(t *testing.T) {
	buf := &bytes.Buffer{}
	data := map[string]any{"key": "value"}
	metadata := map[string]any{"meta": "data"}

	err := WriteSuccess(buf, data, metadata)
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
	err := schwabErrors.NewAuthRequiredError("auth required", errors.New("no token"))

	writeErr := WriteError(buf, err)
	require.NoError(t, writeErr)

	// Verify JSON structure
	var raw map[string]any
	unmarshalErr := json.Unmarshal(buf.Bytes(), &raw)
	require.NoError(t, unmarshalErr)

	assert.Contains(t, raw, "error")
	errorObj := raw["error"].(map[string]any)
	assert.Contains(t, errorObj, "code")
	assert.Contains(t, errorObj, "message")
}

func TestWriteSuccessWithEmptyMetadata(t *testing.T) {
	buf := &bytes.Buffer{}
	data := map[string]any{"test": "data"}
	metadata := map[string]any{}

	err := WriteSuccess(buf, data, metadata)
	require.NoError(t, err)

	var envelope Envelope
	err = json.Unmarshal(buf.Bytes(), &envelope)
	require.NoError(t, err)

	assert.Equal(t, data, envelope.Data)
	assert.Equal(t, metadata, envelope.Metadata)
}

func TestWritePartialMultipleErrors(t *testing.T) {
	buf := &bytes.Buffer{}
	data := map[string]any{"partial": "result"}
	errs := []string{"error 1", "error 2", "error 3"}
	metadata := map[string]any{"count": 3}

	writeErr := WritePartial(buf, data, errs, metadata)
	require.NoError(t, writeErr)

	var envelope Envelope
	unmarshalErr := json.Unmarshal(buf.Bytes(), &envelope)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, 3, len(envelope.Errors))
	assert.Equal(t, errs, envelope.Errors)
}
