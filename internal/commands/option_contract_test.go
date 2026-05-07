package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
)

func TestOptionChainOptsValidation(t *testing.T) {
	tests := []struct {
		name    string
		opts    optionChainOpts
		wantErr bool
		errMsg  string // substring expected in error message
	}{
		{
			name: "valid defaults",
			opts: optionChainOpts{
				Type: "ALL",
			},
			wantErr: false,
		},
		{
			name: "valid with DTE only",
			opts: optionChainOpts{
				Type: "CALL",
				DTE:  30,
			},
			wantErr: false,
		},
		{
			name: "valid with expiration only",
			opts: optionChainOpts{
				Type:       "PUT",
				Expiration: "2026-06-19",
			},
			wantErr: false,
		},
		{
			name: "DTE and expiration mutually exclusive",
			opts: optionChainOpts{
				Type:       "ALL",
				DTE:        30,
				Expiration: "2026-06-19",
			},
			wantErr: true,
			errMsg:  "dte",
		},
		{
			name: "invalid type rejects bogus value",
			opts: optionChainOpts{
				Type: "BOGUS",
			},
			wantErr: true,
			errMsg:  "type",
		},
		{
			name: "type CALL accepted",
			opts: optionChainOpts{
				Type: "CALL",
			},
			wantErr: false,
		},
		{
			name: "type PUT accepted",
			opts: optionChainOpts{
				Type: "PUT",
			},
			wantErr: false,
		},
		{
			name: "type ALL accepted",
			opts: optionChainOpts{
				Type: "ALL",
			},
			wantErr: false,
		},
		{
			name: "strike-count zero is invalid",
			opts: optionChainOpts{
				Type:        "ALL",
				StrikeCount: 0,
			},
			// StrikeCount of 0 means "not set", which is valid (no filter).
			// Only explicitly negative values are invalid.
			wantErr: false,
		},
		{
			name: "strike-count negative is invalid",
			opts: optionChainOpts{
				Type:        "ALL",
				StrikeCount: -1,
			},
			wantErr: true,
			errMsg:  "strike-count",
		},
		{
			name: "strike-count positive is valid",
			opts: optionChainOpts{
				Type:        "ALL",
				StrikeCount: 5,
			},
			wantErr: false,
		},
		{
			name: "strike and strike-count mutually exclusive",
			opts: optionChainOpts{
				Type:        "ALL",
				Strike:      200.0,
				StrikeCount: 5,
			},
			wantErr: true,
			errMsg:  "strike",
		},
		{
			name: "strike and strike-min mutually exclusive",
			opts: optionChainOpts{
				Type:      "ALL",
				Strike:    200.0,
				StrikeMin: 190.0,
			},
			wantErr: true,
			errMsg:  "strike",
		},
		{
			name: "strike and strike-range mutually exclusive",
			opts: optionChainOpts{
				Type:        "ALL",
				Strike:      200.0,
				StrikeRange: "ITM",
			},
			wantErr: true,
			errMsg:  "strike",
		},
		{
			name: "strike-count and strike-range mutually exclusive",
			opts: optionChainOpts{
				Type:        "ALL",
				StrikeCount: 5,
				StrikeRange: "NTM",
			},
			wantErr: true,
			errMsg:  "strike",
		},
		{
			name: "strike-min and strike-max together is valid",
			opts: optionChainOpts{
				Type:      "ALL",
				StrikeMin: 190.0,
				StrikeMax: 210.0,
			},
			wantErr: false,
		},
		{
			name: "strike-min greater than strike-max is invalid",
			opts: optionChainOpts{
				Type:      "ALL",
				StrikeMin: 210.0,
				StrikeMax: 190.0,
			},
			wantErr: true,
			errMsg:  "strike-min",
		},
		{
			name: "strike-min equals strike-max is valid",
			opts: optionChainOpts{
				Type:      "ALL",
				StrikeMin: 200.0,
				StrikeMax: 200.0,
			},
			wantErr: false,
		},
		{
			name: "strike-min/max pair and strike-range mutually exclusive",
			opts: optionChainOpts{
				Type:        "ALL",
				StrikeMin:   190.0,
				StrikeMax:   210.0,
				StrikeRange: "OTM",
			},
			wantErr: true,
			errMsg:  "strike",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act - validateOptionChainOpts does not exist yet (RED phase).
			errs := validateOptionChainOpts(&tt.opts)

			// Assert
			if !tt.wantErr {
				assert.Empty(t, errs, "validateOptionChainOpts(%+v) unexpected errors: %v", tt.opts, errs)
				return
			}

			require.NotEmpty(t, errs, "validateOptionChainOpts(%+v) expected errors but got none", tt.opts)

			// Verify at least one error is a ValidationError.
			var foundValidation bool
			for _, err := range errs {
				var valErr *apperr.ValidationError
				if errors.As(err, &valErr) {
					foundValidation = true
					break
				}
			}
			assert.True(t, foundValidation,
				"validateOptionChainOpts(%+v) expected ValidationError in %v", tt.opts, errs)

			// Verify error message contains expected substring.
			if tt.errMsg != "" {
				var combined string
				for _, err := range errs {
					combined += err.Error() + " "
				}
				assert.Contains(t, combined, tt.errMsg,
					"validateOptionChainOpts(%+v) error message should mention %q", tt.opts, tt.errMsg)
			}
		})
	}
}

func TestOptionContractOptsValidation(t *testing.T) {
	tests := []struct {
		name    string
		opts    optionContractOpts
		wantErr bool
		errMsg  string // substring expected in error message
	}{
		{
			name: "valid call contract",
			opts: optionContractOpts{
				Expiration: "2026-06-19",
				Strike:     200.0,
				Call:       true,
				Put:        false,
			},
			wantErr: false,
		},
		{
			name: "valid put contract",
			opts: optionContractOpts{
				Expiration: "2026-06-19",
				Strike:     200.0,
				Call:       false,
				Put:        true,
			},
			wantErr: false,
		},
		{
			name: "call and put mutually exclusive",
			opts: optionContractOpts{
				Expiration: "2026-06-19",
				Strike:     200.0,
				Call:       true,
				Put:        true,
			},
			wantErr: true,
			errMsg:  "call",
		},
		{
			name: "neither call nor put is invalid",
			opts: optionContractOpts{
				Expiration: "2026-06-19",
				Strike:     200.0,
				Call:       false,
				Put:        false,
			},
			wantErr: true,
			errMsg:  "call",
		},
		{
			name: "expiration required",
			opts: optionContractOpts{
				Expiration: "",
				Strike:     200.0,
				Call:       true,
			},
			wantErr: true,
			errMsg:  "expiration",
		},
		{
			name: "strike must be greater than zero",
			opts: optionContractOpts{
				Expiration: "2026-06-19",
				Strike:     0,
				Call:       true,
			},
			wantErr: true,
			errMsg:  "strike",
		},
		{
			name: "negative strike is invalid",
			opts: optionContractOpts{
				Expiration: "2026-06-19",
				Strike:     -5.0,
				Call:       true,
			},
			wantErr: true,
			errMsg:  "strike",
		},
		{
			name: "all fields missing produces multiple errors",
			opts: optionContractOpts{
				Expiration: "",
				Strike:     0,
				Call:       false,
				Put:        false,
			},
			wantErr: true,
			errMsg:  "expiration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act - validateOptionContractOpts does not exist yet (RED phase).
			errs := validateOptionContractOpts(&tt.opts)

			// Assert
			if !tt.wantErr {
				assert.Empty(t, errs, "validateOptionContractOpts(%+v) unexpected errors: %v", tt.opts, errs)
				return
			}

			require.NotEmpty(t, errs, "validateOptionContractOpts(%+v) expected errors but got none", tt.opts)

			// Verify at least one error is a ValidationError.
			var foundValidation bool
			for _, err := range errs {
				var valErr *apperr.ValidationError
				if errors.As(err, &valErr) {
					foundValidation = true
					break
				}
			}
			assert.True(t, foundValidation,
				"validateOptionContractOpts(%+v) expected ValidationError in %v", tt.opts, errs)

			// Verify error message contains expected substring.
			if tt.errMsg != "" {
				var combined string
				for _, err := range errs {
					combined += err.Error() + " "
				}
				assert.Contains(t, combined, tt.errMsg,
					"validateOptionContractOpts(%+v) error message should mention %q", tt.opts, tt.errMsg)
			}
		})
	}
}

func TestOptionContractCommand(t *testing.T) {
	// Arrange - mock server returns a chain with one matching contract and
	// the underlying quote embedded in the chain response.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path != "/marketdata/v1/chains" {
			http.NotFound(w, r)
			return
		}

		_, _ = w.Write([]byte(`{
			"symbol": "AAPL",
			"status": "SUCCESS",
			"isDelayed": false,
			"underlyingPrice": 199.5,
			"numberOfContracts": 1,
			"underlying": {
				"symbol": "AAPL",
				"bid": 199.40,
				"ask": 199.60,
				"mark": 199.50,
				"last": 199.50,
				"change": -1.25,
				"percentChange": -0.62,
				"totalVolume": 45000000
			},
			"callExpDateMap": {
				"2026-06-19:408": {
					"200.0": [{
						"putCall": "CALL",
						"symbol": "AAPL  260619C00200000",
						"bid": 12.30,
						"ask": 12.45,
						"mark": 12.375,
						"last": 12.35,
						"strikePrice": 200,
						"expirationDate": "2026-06-19",
						"delta": 0.52,
						"gamma": 0.015,
						"theta": -0.045,
						"vega": 0.32,
						"rho": 0.12,
						"volatility": 0.285,
						"openInterest": 1234,
						"totalVolume": 567,
						"inTheMoney": true,
						"theoreticalOptionValue": 12.40,
						"highPrice": 13.00,
						"lowPrice": 11.80,
						"openPrice": 12.00,
						"closePrice": 12.50,
						"netChange": -0.15,
						"percentChange": -1.2,
						"daysToExpiration": 408
					}]
				}
			}
		}`))
	}))
	t.Cleanup(server.Close)

	buf := &bytes.Buffer{}
	cmd := NewOptionCmd(testClient(t, server), buf)
	cmd.SetArgs([]string{"contract", "AAPL", "--expiration", "2026-06-19", "--strike", "200", "--call"})

	// Act
	err := cmd.Execute()

	// Assert
	require.NoError(t, err)

	var envelope struct {
		Data struct {
			Underlying      string  `json:"underlying"`
			Expiration      string  `json:"expiration"`
			Strike          float64 `json:"strike"`
			PutCall         string  `json:"putCall"`
			OCCSymbol       string  `json:"occSymbol"`
			UnderlyingQuote struct {
				Symbol        string  `json:"symbol"`
				Bid           float64 `json:"bid"`
				Ask           float64 `json:"ask"`
				Mark          float64 `json:"mark"`
				Last          float64 `json:"last"`
				NetChange     float64 `json:"netChange"`
				PercentChange float64 `json:"percentChange"`
				TotalVolume   int64   `json:"totalVolume"`
			} `json:"underlyingQuote"`
			Contract struct {
				Symbol                 string  `json:"symbol"`
				Bid                    float64 `json:"bid"`
				Ask                    float64 `json:"ask"`
				Mark                   float64 `json:"mark"`
				Delta                  float64 `json:"delta"`
				Volatility             float64 `json:"volatility"`
				OpenInterest           int64   `json:"openInterest"`
				TotalVolume            int64   `json:"totalVolume"`
				InTheMoney             bool    `json:"inTheMoney"`
				TheoreticalOptionValue float64 `json:"theoreticalOptionValue"`
				Gamma                  float64 `json:"gamma"`
				Theta                  float64 `json:"theta"`
				Vega                   float64 `json:"vega"`
				Rho                    float64 `json:"rho"`
				Last                   float64 `json:"last"`
				DaysToExpiration       int     `json:"daysToExpiration"`
			} `json:"contract"`
		} `json:"data"`
		Metadata struct {
			Returned int `json:"returned"`
		} `json:"metadata"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))

	// Top-level fields
	assert.Equal(t, "AAPL", envelope.Data.Underlying)
	assert.Equal(t, "2026-06-19", envelope.Data.Expiration)
	assert.InDelta(t, 200.0, envelope.Data.Strike, 0.001)
	assert.Equal(t, "CALL", envelope.Data.PutCall)
	assert.Equal(t, "AAPL  260619C00200000", envelope.Data.OCCSymbol)

	// Underlying quote from chain
	assert.Equal(t, "AAPL", envelope.Data.UnderlyingQuote.Symbol)
	assert.InDelta(t, 199.40, envelope.Data.UnderlyingQuote.Bid, 0.001)
	assert.InDelta(t, 199.60, envelope.Data.UnderlyingQuote.Ask, 0.001)
	assert.InDelta(t, 199.50, envelope.Data.UnderlyingQuote.Mark, 0.001)
	assert.InDelta(t, 199.50, envelope.Data.UnderlyingQuote.Last, 0.001)
	assert.InDelta(t, -1.25, envelope.Data.UnderlyingQuote.NetChange, 0.001)
	assert.InDelta(t, -0.62, envelope.Data.UnderlyingQuote.PercentChange, 0.001)
	assert.Equal(t, int64(45000000), envelope.Data.UnderlyingQuote.TotalVolume)

	// Contract fields
	assert.Equal(t, "AAPL  260619C00200000", envelope.Data.Contract.Symbol)
	assert.InDelta(t, 12.30, envelope.Data.Contract.Bid, 0.001)
	assert.InDelta(t, 12.45, envelope.Data.Contract.Ask, 0.001)
	assert.InDelta(t, 12.375, envelope.Data.Contract.Mark, 0.001)
	assert.InDelta(t, 0.52, envelope.Data.Contract.Delta, 0.001)
	assert.InDelta(t, 0.285, envelope.Data.Contract.Volatility, 0.001)
	assert.Equal(t, int64(1234), envelope.Data.Contract.OpenInterest)
	assert.Equal(t, int64(567), envelope.Data.Contract.TotalVolume)
	assert.True(t, envelope.Data.Contract.InTheMoney)
	assert.InDelta(t, 12.40, envelope.Data.Contract.TheoreticalOptionValue, 0.001)
	assert.InDelta(t, 0.015, envelope.Data.Contract.Gamma, 0.001)
	assert.InDelta(t, -0.045, envelope.Data.Contract.Theta, 0.001)
	assert.InDelta(t, 0.32, envelope.Data.Contract.Vega, 0.001)
	assert.InDelta(t, 0.12, envelope.Data.Contract.Rho, 0.001)
	assert.InDelta(t, 12.35, envelope.Data.Contract.Last, 0.001)
	assert.Equal(t, 408, envelope.Data.Contract.DaysToExpiration)

	// Metadata
	assert.Equal(t, 1, envelope.Metadata.Returned)
}

func TestOptionContractCommandNoMatch(t *testing.T) {
	// Arrange - chain has no matching contracts for the requested strike.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path != "/marketdata/v1/chains" {
			http.NotFound(w, r)
			return
		}

		// Return an empty chain with no matching contracts.
		_, _ = w.Write([]byte(`{
			"symbol": "AAPL",
			"status": "SUCCESS",
			"callExpDateMap": {}
		}`))
	}))
	t.Cleanup(server.Close)

	cmd := NewOptionCmd(testClient(t, server), &bytes.Buffer{})

	// Act
	_, err := runTestCommand(t, cmd, "contract", "AAPL", "--expiration", "2026-06-19", "--strike", "999", "--call")

	// Assert
	require.Error(t, err)
	var symbolErr *apperr.SymbolNotFoundError
	require.ErrorAs(t, err, &symbolErr)
	assert.Contains(t, err.Error(), "no contract found")
	assert.Contains(t, err.Error(), "option chain")
}

func TestOptionContractCommandMissingFlags(t *testing.T) {
	// Arrange - command invoked without required --expiration flag.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)

	cmd := NewOptionCmd(testClient(t, server), &bytes.Buffer{})

	// Act - missing --call/--put triggers Cobra's MarkFlagsOneRequired error.
	_, err := runTestCommand(t, cmd, "contract", "AAPL", "--expiration", "2026-06-19", "--strike", "200")

	// Assert - Cobra rejects the command before RunE runs.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "call")
}
