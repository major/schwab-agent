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

func TestNewHistoryCmd_Get_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/marketdata/v1/pricehistory")
		assert.Equal(t, "AAPL", r.URL.Query().Get("symbol"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"symbol":"AAPL","empty":false,"candles":[{"open":148.0,"close":150.25}]}`))
	}))
	defer srv.Close()

	cmd := NewHistoryCmd(testClientWithMarketData(t, srv), &bytes.Buffer{})
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
	cmd := NewHistoryCmd(testClientWithMarketData(t, srv), &buf)
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
	cmd := NewHistoryCmd(testClientWithMarketData(t, srv), &buf)
	_, err := runTestCommand(t, cmd,
		"get",
		"--from", "1700000000000",
		"--to", "1700100000000",
		"AAPL",
	)
	require.NoError(t, err)
}

func TestNewHistoryCmd_Get_InvalidNumericFlag(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"symbol":"AAPL","empty":false,"candles":[]}`))
	}))
	defer srv.Close()

	cmd := NewHistoryCmd(testClientWithMarketData(t, srv), &bytes.Buffer{})
	_, err := runTestCommand(t, cmd, "get", "--period", "not-a-number", "AAPL")
	require.Error(t, err)
	var valErr *apperr.ValidationError
	require.ErrorAs(t, err, &valErr)
	require.ErrorContains(t, err, "invalid period: not-a-number")
	assert.Zero(t, requestCount, "invalid local flags should fail before calling Schwab")
}

func TestNewHistoryCmd_Get_InvalidEnumFlag(t *testing.T) {
	server := jsonServer(`{}`)
	defer server.Close()

	cmd := NewHistoryCmd(testClientWithMarketData(t, server), &bytes.Buffer{})
	_, err := runTestCommand(t, cmd, "get", "--period-type", "decade", "AAPL")
	require.Error(t, err)
	var valErr *apperr.ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.ErrorContains(t, err, "invalid period-type")
}

func TestNewPriceHistoryParams_InvalidNumericFields(t *testing.T) {
	tests := []struct {
		name    string
		input   priceHistoryParamsInput
		message string
	}{
		{
			name:    "invalid frequency",
			input:   priceHistoryParamsInput{Frequency: "daily"},
			message: "invalid frequency: daily",
		},
		{
			name:    "zero period",
			input:   priceHistoryParamsInput{Period: "0"},
			message: "invalid period: 0 (must be > 0)",
		},
		{
			name:    "negative frequency",
			input:   priceHistoryParamsInput{Frequency: "-1"},
			message: "invalid frequency: -1 (must be > 0)",
		},
		{
			name:    "invalid from",
			input:   priceHistoryParamsInput{StartDate: "yesterday"},
			message: "invalid from: yesterday",
		},
		{
			name:    "invalid to",
			input:   priceHistoryParamsInput{EndDate: "tomorrow"},
			message: "invalid to: tomorrow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newPriceHistoryParams(tt.input)
			require.Error(t, err)
			var valErr *apperr.ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.ErrorContains(t, err, tt.message)
		})
	}
}

func TestNewHistoryCmd_Get_MissingSymbol(t *testing.T) {
	server := jsonServer(`{}`)
	defer server.Close()

	cmd := NewHistoryCmd(testClientWithMarketData(t, server), &bytes.Buffer{})
	_, err := runTestCommand(t, cmd, "get")
	require.Error(t, err)
}

func TestNewHistoryCmd_Get_ExtraArgs(t *testing.T) {
	server := jsonServer(`{}`)
	defer server.Close()

	// "history get AAPL MSFT" should reject the extra positional arg
	// since the command only accepts a single symbol.
	cmd := NewHistoryCmd(testClientWithMarketData(t, server), &bytes.Buffer{})
	_, err := runTestCommand(t, cmd, "get", "AAPL", "MSFT")
	require.Error(t, err)
}

func TestNewHistoryCmd_Get_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"server error"}`))
	}))
	defer srv.Close()

	cmd := NewHistoryCmd(testClientWithMarketData(t, srv), &bytes.Buffer{})
	_, err := runTestCommand(t, cmd, "get", "AAPL")
	require.Error(t, err)
}

func TestNewHistoryCmd_PriceHistoryAlias(t *testing.T) {
	// Verify that the "price-history" alias is registered on the history command.
	srv := jsonServer(`{"symbol":"AAPL","empty":false,"candles":[]}`)
	defer srv.Close()

	cmd := NewHistoryCmd(testClientWithMarketData(t, srv), &bytes.Buffer{})

	assert.Contains(t, cmd.Aliases, "price-history")
}
