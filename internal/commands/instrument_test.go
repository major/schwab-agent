package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/output"
)

func TestNewInstrumentCmd_Search_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/marketdata/v1/instruments", r.URL.Path)
		assert.Equal(t, "AAPL", r.URL.Query().Get("symbol"))
		assert.Equal(t, "symbol-search", r.URL.Query().Get("projection"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"instruments":[{"cusip":"037833100","symbol":"AAPL","description":"Apple Inc"}]}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	cmd := NewInstrumentCmd(testClientWithMarketData(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "search", "AAPL")
	require.NoError(t, err)

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	assert.NotNil(t, envelope.Data)
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
}

func TestNewInstrumentCmd_Search_WithProjection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "fundamental", r.URL.Query().Get("projection"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"instruments":[{"cusip":"037833100","symbol":"AAPL"}]}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	cmd := NewInstrumentCmd(testClientWithMarketData(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "search", "--projection", "fundamental", "AAPL")
	require.NoError(t, err)

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	assert.NotNil(t, envelope.Data)
}

func TestNewInstrumentCmd_Search_MissingQuery(t *testing.T) {
	server := jsonServer(`{}`)
	defer server.Close()

	var buf bytes.Buffer
	cmd := NewInstrumentCmd(testClient(t, server), &buf)
	_, err := runTestCommand(t, cmd, "search")
	require.Error(t, err)
	// cobra.ExactArgs(1) produces a standard error, not ValidationError
	assert.ErrorContains(t, err, "accepts 1 arg(s), received 0")
}

func TestNewInstrumentCmd_Search_InvalidProjection(t *testing.T) {
	server := jsonServer(`{}`)
	defer server.Close()

	var buf bytes.Buffer
	cmd := NewInstrumentCmd(testClient(t, server), &buf)
	_, err := runTestCommand(t, cmd, "search", "--projection", "unknown", "AAPL")
	require.Error(t, err)
	var validationErr *apperr.ValidationError
	require.ErrorAs(t, err, &validationErr)
}

func TestNewInstrumentCmd_Search_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"server unavailable"}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	cmd := NewInstrumentCmd(testClientWithMarketData(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "search", "AAPL")
	require.Error(t, err)
	var httpErr *apperr.HTTPError
	require.ErrorAs(t, err, &httpErr)
}

func TestNewInstrumentCmd_Get_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/marketdata/v1/instruments/037833100", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"instruments":[{"cusip":"037833100","symbol":"AAPL","description":"Apple Inc","exchange":"NASDAQ"}]
		}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	cmd := NewInstrumentCmd(testClientWithMarketData(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "get", "037833100")
	require.NoError(t, err)

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	assert.NotNil(t, envelope.Data)
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
}

func TestNewInstrumentCmd_Get_EmptyInstrumentResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/marketdata/v1/instruments/000000000", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"instruments":[]}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	cmd := NewInstrumentCmd(testClientWithMarketData(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "get", "000000000")
	require.Error(t, err)
	var notFoundErr *apperr.SymbolNotFoundError
	require.ErrorAs(t, err, &notFoundErr)
}

func TestNewInstrumentCmd_Get_MissingCUSIP(t *testing.T) {
	server := jsonServer(`{}`)
	defer server.Close()

	var buf bytes.Buffer
	cmd := NewInstrumentCmd(testClient(t, server), &buf)
	_, err := runTestCommand(t, cmd, "get")
	require.Error(t, err)
	// cobra.ExactArgs(1) produces a standard error, not ValidationError
	assert.ErrorContains(t, err, "accepts 1 arg(s), received 0")
}

func TestNewInstrumentCmd_Get_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	cmd := NewInstrumentCmd(testClientWithMarketData(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "get", "000000000")
	require.Error(t, err)
	var notFoundErr *apperr.SymbolNotFoundError
	require.ErrorAs(t, err, &notFoundErr)
}

func TestNewInstrumentCmd_Get_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"server unavailable"}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	cmd := NewInstrumentCmd(testClientWithMarketData(t, srv), &buf)
	_, err := runTestCommand(t, cmd, "get", "000000000")
	require.Error(t, err)
	var httpErr *apperr.HTTPError
	require.ErrorAs(t, err, &httpErr)
}

func TestMapInstrumentGetError(t *testing.T) {
	plainErr := assert.AnError

	tests := []struct {
		name        string
		input       error
		expectedErr error
	}{
		{
			name:        "nil returns nil",
			input:       nil,
			expectedErr: nil,
		},
		{
			name:        "plain error passes through",
			input:       plainErr,
			expectedErr: plainErr,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mapInstrumentGetError(tc.input)
			if tc.expectedErr == nil {
				require.NoError(t, got)
				return
			}

			require.Same(t, tc.expectedErr, got)
		})
	}
}
