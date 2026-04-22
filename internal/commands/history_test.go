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

func TestHistoryCommand_Get_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/marketdata/v1/pricehistory")
		assert.Equal(t, "AAPL", r.URL.Query().Get("symbol"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"symbol":"AAPL","empty":false,"candles":[{"open":148.0,"close":150.25}]}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	cmd := HistoryCommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd, "history", "get", "AAPL"))

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	assert.NotNil(t, envelope.Data)
	assert.Contains(t, envelope.Metadata, "timestamp")
}

func TestHistoryCommand_Get_WithFlags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		assert.Equal(t, "AAPL", q.Get("symbol"))
		assert.Equal(t, "day", q.Get("periodType"))
		assert.Equal(t, "10", q.Get("period"))
		assert.Equal(t, "minute", q.Get("frequencyType"))
		assert.Equal(t, "5", q.Get("frequency"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"symbol":"AAPL","empty":false,"candles":[]}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	cmd := HistoryCommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd,
		"history", "get",
		"--period-type", "day",
		"--period", "10",
		"--frequency-type", "minute",
		"--frequency", "5",
		"AAPL",
	))

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	assert.NotNil(t, envelope.Data)
}

func TestHistoryCommand_Get_DateRange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		assert.Equal(t, "1700000000000", q.Get("startDate"))
		assert.Equal(t, "1700100000000", q.Get("endDate"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"symbol":"AAPL","empty":false,"candles":[]}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	cmd := HistoryCommand(testClient(t, srv), &buf)
	require.NoError(t, runTestCommand(t, cmd,
		"history", "get",
		"--from", "1700000000000",
		"--to", "1700100000000",
		"AAPL",
	))
}

func TestHistoryCommand_Get_MissingSymbol(t *testing.T) {
	server := jsonServer(`{}`)
	defer server.Close()

	var buf bytes.Buffer
	cmd := HistoryCommand(testClient(t, server), &buf)
	err := runTestCommand(t, cmd, "history", "get")
	require.Error(t, err)

	var valErr *schwabErrors.ValidationError
	assert.ErrorAs(t, err, &valErr)
}

func TestHistoryCommand_Get_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"server error"}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	cmd := HistoryCommand(testClient(t, srv), &buf)
	err := runTestCommand(t, cmd, "history", "get", "AAPL")
	require.Error(t, err)
}
