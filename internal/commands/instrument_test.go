package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	schwabErrors "github.com/major/schwab-agent/internal/errors"
	"github.com/major/schwab-agent/internal/output"
)

func TestInstrumentCommand_Search_Success(t *testing.T) {
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
	cmd := InstrumentCommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "instrument", "search", "AAPL"))

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	assert.NotNil(t, envelope.Data)
	assert.Contains(t, envelope.Metadata, "timestamp")
}

func TestInstrumentCommand_Search_WithProjection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "fundamental", r.URL.Query().Get("projection"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"instruments":[{"cusip":"037833100","symbol":"AAPL"}]}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	cmd := InstrumentCommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd,
		"instrument", "search",
		"--projection", "fundamental",
		"AAPL",
	))

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	assert.NotNil(t, envelope.Data)
}

func TestInstrumentCommand_Search_MissingQuery(t *testing.T) {
	server := jsonServer(`{}`)
	defer server.Close()

	var buf bytes.Buffer
	cmd := InstrumentCommand(testClient(t, server), &buf)
	err := runTestCommand(t, cmd, "instrument", "search")
	require.Error(t, err)

	var valErr *schwabErrors.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestInstrumentCommand_Get_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/marketdata/v1/instruments/037833100", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		// The Schwab API returns {"instruments": [...]} even for single-CUSIP lookups.
		_, _ = w.Write([]byte(`{"instruments":[{"cusip":"037833100","symbol":"AAPL","description":"Apple Inc","exchange":"NASDAQ"}]}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	cmd := InstrumentCommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "instrument", "get", "037833100"))

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	assert.NotNil(t, envelope.Data)
	assert.Contains(t, envelope.Metadata, "timestamp")
}

func TestInstrumentCommand_Get_MissingCUSIP(t *testing.T) {
	server := jsonServer(`{}`)
	defer server.Close()

	var buf bytes.Buffer
	cmd := InstrumentCommand(testClient(t, server), &buf)
	err := runTestCommand(t, cmd, "instrument", "get")
	require.Error(t, err)

	var valErr *schwabErrors.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestInstrumentCommand_Get_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	cmd := InstrumentCommand(testClient(t, srv), &buf)
	err := runTestCommand(t, cmd, "instrument", "get", "000000000")
	require.Error(t, err)
}
