package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	schwabErrors "github.com/major/schwab-agent/internal/errors"
	"github.com/major/schwab-agent/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChainGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/marketdata/v1/chains" {
			assert.Equal(t, "AAPL", r.URL.Query().Get("symbol"))
			_, _ = w.Write([]byte(`{"symbol":"AAPL","status":"SUCCESS","underlyingPrice":150.0}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	var buf bytes.Buffer
	cmd := ChainCommand(testClient(t, server), &buf)
	require.NoError(t, runTestCommand(t, cmd, "chain", "get", "AAPL"))

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "AAPL", data["symbol"])
	assert.NotNil(t, envelope.Metadata["timestamp"])
}

func TestChainGetWithFlags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/marketdata/v1/chains" {
			q := r.URL.Query()
			assert.Equal(t, "AAPL", q.Get("symbol"))
			assert.Equal(t, "CALL", q.Get("contractType"))
			assert.Equal(t, "10", q.Get("strikeCount"))
			assert.Equal(t, "SINGLE", q.Get("strategy"))
			assert.Equal(t, "2024-01-01", q.Get("fromDate"))
			assert.Equal(t, "2024-12-31", q.Get("toDate"))
			_, _ = w.Write([]byte(`{"symbol":"AAPL","status":"SUCCESS"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	var buf bytes.Buffer
	cmd := ChainCommand(testClient(t, server), &buf)
	require.NoError(t, runTestCommand(t, cmd, "chain", "get",
		"--type", "CALL",
		"--strike-count", "10",
		"--strategy", "SINGLE",
		"--from-date", "2024-01-01",
		"--to-date", "2024-12-31",
		"AAPL",
	))

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	assert.NotNil(t, envelope.Data)
	assert.NotNil(t, envelope.Metadata["timestamp"])
}

func TestChainGetAdvancedFlags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/marketdata/v1/chains" {
			q := r.URL.Query()
			assert.Equal(t, "AAPL", q.Get("symbol"))
			assert.Equal(t, "ANALYTICAL", q.Get("strategy"))
			assert.Equal(t, "true", q.Get("includeUnderlyingQuote"))
			assert.Equal(t, "5.0", q.Get("interval"))
			assert.Equal(t, "150.0", q.Get("strike"))
			assert.Equal(t, "NTM", q.Get("range"))
			assert.Equal(t, "30.5", q.Get("volatility"))
			assert.Equal(t, "148.50", q.Get("underlyingPrice"))
			assert.Equal(t, "4.5", q.Get("interestRate"))
			assert.Equal(t, "45", q.Get("daysToExpiration"))
			_, _ = w.Write([]byte(`{"symbol":"AAPL","status":"SUCCESS"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	var buf bytes.Buffer
	cmd := ChainCommand(testClient(t, server), &buf)
	require.NoError(t, runTestCommand(t, cmd, "chain", "get",
		"--strategy", "ANALYTICAL",
		"--include-underlying-quote",
		"--interval", "5.0",
		"--strike", "150.0",
		"--strike-range", "NTM",
		"--volatility", "30.5",
		"--underlying-price", "148.50",
		"--interest-rate", "4.5",
		"--days-to-expiration", "45",
		"AAPL",
	))

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	assert.NotNil(t, envelope.Data)
}

func TestChainExpiration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/marketdata/v1/expirationchain" {
			assert.Equal(t, "AAPL", r.URL.Query().Get("symbol"))
			_, _ = w.Write([]byte(`{"expirationList":[{"expirationDate":"2024-01-19"},{"expirationDate":"2024-02-16"}]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	var buf bytes.Buffer
	cmd := ChainCommand(testClient(t, server), &buf)
	require.NoError(t, runTestCommand(t, cmd, "chain", "expiration", "AAPL"))

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	expList, ok := data["expirationList"].([]any)
	require.True(t, ok)
	assert.Len(t, expList, 2)
	assert.NotNil(t, envelope.Metadata["timestamp"])
}

func TestChainGetNoArgs(t *testing.T) {
	server := jsonServer(`{}`)
	defer server.Close()

	var buf bytes.Buffer
	cmd := ChainCommand(testClient(t, server), &buf)
	err := runTestCommand(t, cmd, "chain", "get")
	require.Error(t, err)

	var valErr *schwabErrors.ValidationError
	assert.ErrorAs(t, err, &valErr)
	assert.Empty(t, buf.String())
}

func TestChainExpirationNoArgs(t *testing.T) {
	server := jsonServer(`{}`)
	defer server.Close()

	var buf bytes.Buffer
	cmd := ChainCommand(testClient(t, server), &buf)
	err := runTestCommand(t, cmd, "chain", "expiration")
	require.Error(t, err)

	var valErr *schwabErrors.ValidationError
	assert.ErrorAs(t, err, &valErr)
	assert.Empty(t, buf.String())
}
