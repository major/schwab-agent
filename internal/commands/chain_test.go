package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-go/schwab/marketdata"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/output"
)

func TestNewChainCmd(t *testing.T) {
	t.Run("chain get with symbol", func(t *testing.T) {
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
		cmd := NewChainCmd(testClient(t, server), &buf)
		_, err := runTestCommand(t, cmd, "get", "AAPL")
		require.NoError(t, err)

		var envelope output.Envelope
		require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
		data, ok := envelope.Data.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "AAPL", data["symbol"])
		assert.NotEmpty(t, envelope.Metadata.Timestamp)
	})

	t.Run("chain get with flags", func(t *testing.T) {
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
		cmd := NewChainCmd(testClient(t, server), &buf)
		_, err := runTestCommand(t, cmd, "get", "AAPL",
			"--type", "CALL",
			"--strike-count", "10",
			"--strategy", "SINGLE",
			"--from-date", "2024-01-01",
			"--to-date", "2024-12-31",
		)
		require.NoError(t, err)

		var envelope output.Envelope
		require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
		assert.NotNil(t, envelope.Data)
		assert.NotEmpty(t, envelope.Metadata.Timestamp)
	})

	t.Run("chain get with advanced flags", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/marketdata/v1/chains" {
				q := r.URL.Query()
				assert.Equal(t, "AAPL", q.Get("symbol"))
				assert.Equal(t, "ANALYTICAL", q.Get("strategy"))
				assert.Equal(t, "true", q.Get("includeUnderlyingQuote"))
				assert.Equal(t, "5", q.Get("interval"))
				assert.Equal(t, "150", q.Get("strike"))
				assert.Equal(t, "NTM", q.Get("range"))
				assert.Equal(t, "30.5", q.Get("volatility"))
				assert.Equal(t, "148.5", q.Get("underlyingPrice"))
				assert.Equal(t, "4.5", q.Get("interestRate"))
				assert.Equal(t, "45", q.Get("daysToExpiration"))
				_, _ = w.Write([]byte(`{"symbol":"AAPL","status":"SUCCESS"}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewChainCmd(testClient(t, server), &buf)
		_, err := runTestCommand(t, cmd, "get", "AAPL",
			"--strategy", "ANALYTICAL",
			"--include-underlying-quote",
			"--interval", "5.0",
			"--strike", "150.0",
			"--strike-range", "NTM",
			"--volatility", "30.5",
			"--underlying-price", "148.50",
			"--interest-rate", "4.5",
			"--days-to-expiration", "45",
		)
		require.NoError(t, err)

		var envelope output.Envelope
		require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
		assert.NotNil(t, envelope.Data)
	})

	t.Run("chain expiration with symbol", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/marketdata/v1/expirationchain" {
				assert.Equal(t, "AAPL", r.URL.Query().Get("symbol"))
				_, _ = w.Write(
					[]byte(`{"expirationList":[{"expiration":"2024-01-19"},{"expiration":"2024-02-16"}]}`),
				)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewChainCmd(testClient(t, server), &buf)
		_, err := runTestCommand(t, cmd, "expiration", "AAPL")
		require.NoError(t, err)

		var envelope output.Envelope
		require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
		data, ok := envelope.Data.(map[string]any)
		require.True(t, ok)
		expList, ok := data["expirationList"].([]any)
		require.True(t, ok)
		assert.Len(t, expList, 2)
		first, ok := expList[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "2024-01-19", first["expirationDate"])
		assert.NotEmpty(t, envelope.Metadata.Timestamp)
	})

	t.Run("chain get without symbol", func(t *testing.T) {
		server := jsonServer(`{}`)
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewChainCmd(testClient(t, server), &buf)
		_, err := runTestCommand(t, cmd, "get")
		require.Error(t, err)
		// cobra.ExactArgs returns a plain error, not ValidationError
		assert.Contains(t, err.Error(), "accepts 1 arg(s), received 0")
	})

	t.Run("chain expiration without symbol", func(t *testing.T) {
		server := jsonServer(`{}`)
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewChainCmd(testClient(t, server), &buf)
		_, err := runTestCommand(t, cmd, "expiration")
		require.Error(t, err)
		// cobra.ExactArgs returns a plain error, not ValidationError
		assert.Contains(t, err.Error(), "accepts 1 arg(s), received 0")
	})

	t.Run("chain without subcommand", func(t *testing.T) {
		server := jsonServer(`{}`)
		defer server.Close()

		var buf bytes.Buffer
		cmd := NewChainCmd(testClient(t, server), &buf)
		_, err := runTestCommand(t, cmd)
		require.Error(t, err)

		var valErr *apperr.ValidationError
		require.ErrorAs(t, err, &valErr)
		assert.Contains(t, valErr.Error(), "requires a subcommand")
	})
}

func TestOptionChainParams(t *testing.T) {
	t.Run("all flags", func(t *testing.T) {
		opts := &chainGetOpts{
			Type:                   chainContractType(marketdata.OptionChainContractTypeCall),
			StrikeCount:            "10",
			Strategy:               chainStrategy(marketdata.OptionChainStrategyAnalytical),
			FromDate:               "2024-01-01",
			ToDate:                 "2024-12-31",
			IncludeUnderlyingQuote: true,
			Interval:               "5.0",
			Strike:                 "150.0",
			StrikeRange:            strikeRange(marketdata.OptionChainRangeNearTheMoney),
			Volatility:             "30.5",
			UnderlyingPrice:        "148.50",
			InterestRate:           "-1.25",
			DaysToExpiration:       "45",
		}

		params, err := optionChainParams("AAPL", opts)
		require.NoError(t, err)

		assert.Equal(t, "AAPL", params.Symbol)
		assert.Equal(t, marketdata.OptionChainContractTypeCall, params.ContractType)
		assert.Equal(t, 10, params.StrikeCount)
		assert.True(t, params.IncludeUnderlyingQuote)
		assert.Equal(t, marketdata.OptionChainStrategyAnalytical, params.Strategy)
		assert.InDelta(t, 5.0, params.Interval, 0.001)
		assert.InDelta(t, 150.0, params.Strike, 0.001)
		assert.Equal(t, marketdata.OptionChainRangeNearTheMoney, params.Range)
		assert.Equal(t, "2024-01-01", params.FromDate)
		assert.Equal(t, "2024-12-31", params.ToDate)
		assert.InDelta(t, 30.5, params.Volatility, 0.001)
		assert.InDelta(t, 148.50, params.UnderlyingPrice, 0.001)
		assert.InDelta(t, -1.25, params.InterestRate, 0.001)
		assert.Equal(t, 45, params.DaysToExpiration)
	})

	t.Run("empty opts only set symbol", func(t *testing.T) {
		params, err := optionChainParams("MSFT", &chainGetOpts{})
		require.NoError(t, err)

		assert.Equal(t, "MSFT", params.Symbol)
		assert.Empty(t, params.ContractType)
		assert.Zero(t, params.StrikeCount)
		assert.Zero(t, params.Interval)
		assert.Zero(t, params.InterestRate)
	})
}

func TestOptionChainParamsValidation(t *testing.T) {
	tests := []struct {
		name string
		opts chainGetOpts
	}{
		{
			name: "invalid strike count",
			opts: chainGetOpts{StrikeCount: "not-a-number"},
		},
		{
			name: "invalid interval",
			opts: chainGetOpts{Interval: "not-a-number"},
		},
		{
			name: "zero interval",
			opts: chainGetOpts{Interval: "0"},
		},
		{
			name: "negative interval",
			opts: chainGetOpts{Interval: "-1"},
		},
		{
			name: "zero strike",
			opts: chainGetOpts{Strike: "0"},
		},
		{
			name: "negative strike",
			opts: chainGetOpts{Strike: "-1"},
		},
		{
			name: "negative volatility",
			opts: chainGetOpts{Volatility: "-1"},
		},
		{
			name: "negative underlying price",
			opts: chainGetOpts{UnderlyingPrice: "-1"},
		},
		{
			name: "invalid interest rate",
			opts: chainGetOpts{InterestRate: "not-a-number"},
		},
		{
			name: "zero interest rate",
			opts: chainGetOpts{InterestRate: "0"},
		},
		{
			name: "invalid days to expiration",
			opts: chainGetOpts{DaysToExpiration: "not-a-number"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := optionChainParams("AAPL", &tt.opts)
			require.Error(t, err)
			var valErr *apperr.ValidationError
			require.ErrorAs(t, err, &valErr)
		})
	}
}
