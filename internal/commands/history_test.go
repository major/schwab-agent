package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/output"
)

func TestNewHistoryCmd_Get_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/marketdata/v1/pricehistory")
		assert.Equal(t, "AAPL", r.URL.Query().Get("symbol"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"symbol":"AAPL","empty":false,"candles":[{"open":148.0,"close":150.25}]}`))
	}))
	defer srv.Close()

	cmd := NewHistoryCmd(testClient(t, srv), &bytes.Buffer{})
	_, err := runTestCommand(t, cmd, "get", "AAPL")
	require.NoError(t, err)
}

func TestNewHistoryCmd_Get_WithFlags(t *testing.T) {
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
	cmd := NewHistoryCmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd,
		"get",
		"--period-type", "day",
		"--period", "10",
		"--frequency-type", "minute",
		"--frequency", "5",
		"AAPL",
	)
	require.NoError(t, err)

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	assert.NotNil(t, envelope.Data)
}

func TestNewHistoryCmd_Get_DateRange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		assert.Equal(t, "1700000000000", q.Get("startDate"))
		assert.Equal(t, "1700100000000", q.Get("endDate"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"symbol":"AAPL","empty":false,"candles":[]}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	cmd := NewHistoryCmd(testClient(t, srv), &buf)
	_, err := runTestCommand(t, cmd,
		"get",
		"--from", "1700000000000",
		"--to", "1700100000000",
		"AAPL",
	)
	require.NoError(t, err)
}

func TestNewHistoryCmd_Get_MissingSymbol(t *testing.T) {
	server := jsonServer(`{}`)
	defer server.Close()

	cmd := NewHistoryCmd(testClient(t, server), &bytes.Buffer{})
	_, err := runTestCommand(t, cmd, "get")
	require.Error(t, err)
}

func TestNewHistoryCmd_Get_ExtraArgs(t *testing.T) {
	server := jsonServer(`{}`)
	defer server.Close()

	// "history get AAPL MSFT" should reject the extra positional arg
	// since the command only accepts a single symbol.
	cmd := NewHistoryCmd(testClient(t, server), &bytes.Buffer{})
	_, err := runTestCommand(t, cmd, "get", "AAPL", "MSFT")
	require.Error(t, err)
}

func TestNewHistoryCmd_Get_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"server error"}`))
	}))
	defer srv.Close()

	cmd := NewHistoryCmd(testClient(t, srv), &bytes.Buffer{})
	_, err := runTestCommand(t, cmd, "get", "AAPL")
	require.Error(t, err)
}

func TestNewHistoryCmd_PriceHistoryAlias(t *testing.T) {
	// Verify that the "price-history" alias is registered on the history command.
	srv := jsonServer(`{"symbol":"AAPL","empty":false,"candles":[]}`)
	defer srv.Close()

	cmd := NewHistoryCmd(testClient(t, srv), &bytes.Buffer{})

	assert.Contains(t, cmd.Aliases, "price-history")
}
