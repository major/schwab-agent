package commands

import (
	"context"
	"testing"
	"time"

	"github.com/major/schwab-go/schwab/marketdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/models"
)

func TestResolveRequestedColumns(t *testing.T) {
	tests := []struct {
		name    string
		fields  string
		want    []string
		wantErr bool
	}{
		{name: "default", fields: "", want: chainDefaultColumns},
		{name: "trimmed", fields: " strike, bid, ,delta ", want: []string{"strike", "bid", "delta"}},
		{name: "unknown", fields: "strike,nope", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveRequestedColumns(tt.fields)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestOptionChainFieldExtractionAndSorting(t *testing.T) {
	contract := fullOptionContractForTest()
	columns := []string{
		"expiry", "strike", "cp", "symbol", "bid", "ask", "mark", "last",
		"delta", "gamma", "theta", "vega", "rho", "iv", "oi", "volume", "itm", "dte", "unknown",
	}

	row := extractContractRow(contract, "2026-06-19", columns)

	assert.Equal(t, "2026-06-19", row.values[0])
	assert.InEpsilon(t, 200.0, row.values[1], 0.0001)
	assert.Equal(t, "CALL", row.values[2])
	assert.Equal(t, "AAPL 260619C00200000", row.values[3])
	assert.InEpsilon(t, 1.1, row.values[4], 0.0001)
	assert.InEpsilon(t, 1.2, row.values[5], 0.0001)
	assert.InEpsilon(t, 1.15, row.values[6], 0.0001)
	assert.InEpsilon(t, 1.05, row.values[7], 0.0001)
	assert.InEpsilon(t, 0.5, row.values[8], 0.0001)
	assert.InEpsilon(t, 0.1, row.values[9], 0.0001)
	assert.InEpsilon(t, -0.2, row.values[10], 0.0001)
	assert.InEpsilon(t, 0.3, row.values[11], 0.0001)
	assert.InEpsilon(t, 0.4, row.values[12], 0.0001)
	assert.InEpsilon(t, 25.0, row.values[13], 0.0001)
	assert.Equal(t, int64(100), row.values[14])
	assert.Equal(t, int64(25), row.values[15])
	assert.Equal(t, true, row.values[16])
	assert.NotNil(t, row.values[17])
	assert.Nil(t, row.values[18])
	assert.Nil(t, computeDTE("not-a-date", time.Now()))

	chain := &models.OptionChain{
		CallExpDateMap: map[string]map[string][]*models.OptionContract{
			"2026-07-17:70": {"205.0": {optionContractForTest("B", "CALL", 205)}},
			"2026-06-19:42": {"200.0": {optionContractForTest("A", "CALL", 200)}},
		},
		PutExpDateMap: map[string]map[string][]*models.OptionContract{
			"2026-06-19:42": {"200.0": {optionContractForTest("C", "PUT", 200)}},
		},
	}
	rows := buildSortedChainRows(chain, []string{"symbol"})

	require.Len(t, rows, 3)
	assert.Equal(
		t,
		[]string{"2026-06-19", "2026-06-19", "2026-07-17"},
		[]string{rows[0].expiry, rows[1].expiry, rows[2].expiry},
	)
	assert.Equal(t, []string{"CALL", "PUT", "CALL"}, []string{rows[0].putCall, rows[1].putCall, rows[2].putCall})
}

func TestOptionChainAgentFieldAliases(t *testing.T) {
	contract := fullOptionContractForTest()
	columns := []string{"mid", "volatility", "openInterest", "totalVolume", "daysToExpiration"}

	row := extractContractRow(contract, "2026-06-19", columns)

	assert.InEpsilon(t, 1.15, row.values[0], 0.0001)
	assert.InEpsilon(t, 25.0, row.values[1], 0.0001)
	assert.Equal(t, int64(100), row.values[2])
	assert.Equal(t, int64(25), row.values[3])
	assert.Equal(t, 42, row.values[4])

	missingSide := &models.OptionContract{Bid: new(1.0)}
	assert.Nil(t, contractFieldValue(missingSide, "mid", "2026-06-19"))
}

func TestOptionChainExpirationHelpers(t *testing.T) {
	chain := &models.OptionChain{
		CallExpDateMap: map[string]map[string][]*models.OptionContract{
			"2026-07-17:70": {},
			"2026-06-19:42": {},
		},
		PutExpDateMap: map[string]map[string][]*models.OptionContract{
			"2026-06-19:42":  {},
			"2026-08-21:105": {},
		},
	}

	assert.Equal(t, []string{"2026-06-19", "2026-07-17", "2026-08-21"}, extractExpirationsFromChain(chain))

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	selected, err := selectNearestExpiration([]string{"2026-06-10", "2026-06-20"}, 14, now)
	require.NoError(t, err)
	assert.Equal(t, "2026-06-10", selected)

	_, err = selectNearestExpiration(nil, 14, now)
	require.Error(t, err)
	_, err = selectNearestExpiration([]string{"bad-date"}, 14, now)
	require.Error(t, err)
	_, err = selectNearestExpiration([]string{"2026-06-10"}, 0, now)
	require.Error(t, err)
}

func TestFlattenExpirationRowsEdgeCases(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	columns, rows, err := flattenExpirationRows(nil, now)
	require.NoError(t, err)
	assert.Equal(t, expirationColumns, columns)
	assert.Empty(t, rows)

	chain := &client.ExpirationChain{ExpirationList: []client.ExpirationDate{
		{ExpirationDate: "2026-06-26"},
		{ExpirationDate: "2026-06-19"},
	}}
	columns, rows, err = flattenExpirationRows(chain, now)
	require.NoError(t, err)
	assert.Equal(t, expirationColumns, columns)
	require.Len(t, rows, 2)
	assert.Equal(t, []any{"2026-06-19", 18, "R", true}, rows[0])
	assert.Equal(t, []any{"2026-06-26", 25, "S", false}, rows[1])

	_, _, err = flattenExpirationRows(
		&client.ExpirationChain{ExpirationList: []client.ExpirationDate{{ExpirationDate: "bad"}}},
		now,
	)
	require.Error(t, err)
}

func TestOptionChainStrikeFilters(t *testing.T) {
	rows := []chainRow{
		{expiry: "2026-06-19", strike: 95, putCall: "CALL"},
		{expiry: "2026-06-19", strike: 100, putCall: "CALL"},
		{expiry: "2026-06-19", strike: 100, putCall: "PUT"},
		{expiry: "2026-07-17", strike: 105, putCall: "CALL"},
		{expiry: "2026-07-17", strike: 110, putCall: "CALL"},
	}
	underlying := 102.5
	chain := &models.OptionChain{UnderlyingPrice: &underlying}

	assert.Len(t, filterRowsByExpiration(rows, "2026-06-19"), 3)
	assert.Len(t, filterRowsByStrike(rows, 100), 2)
	assert.Equal(t, rows[:2], filterRowsByStrikeCount(rows[:2], 5, chain))

	countFiltered := filterRowsByStrikeCount(rows, 2, chain)
	require.Len(t, countFiltered, 3)
	assert.Equal(
		t,
		[]float64{100, 100, 105},
		[]float64{countFiltered[0].strike, countFiltered[1].strike, countFiltered[2].strike},
	)

	assert.Len(t, filterRowsByStrikeRange(rows, 100, 105), 3)
	assert.Len(t, filterRowsByStrikeRange(rows, 100, 0), 4)
	assert.Len(t, filterRowsByStrikeRange(rows, 0, 105), 4)

	assert.Len(t, applyStrikeSelectors(rows, &optionChainOpts{Strike: 100}, chain), 2)
	assert.Len(t, applyStrikeSelectors(rows, &optionChainOpts{StrikeCount: 1}, chain), 2)
	assert.Len(t, applyStrikeSelectors(rows, &optionChainOpts{StrikeMin: 100, StrikeMax: 105}, chain), 3)
	assert.Equal(t, rows, applyStrikeSelectors(rows, &optionChainOpts{StrikeRange: strikeRangeNTM}, chain))
}

func TestOptionChainDeltaFilters(t *testing.T) {
	rows := []chainRow{
		{strike: 90, contract: &models.OptionContract{Delta: new(-0.30)}},
		{strike: 92, contract: &models.OptionContract{Delta: new(-0.25)}},
		{strike: 95, contract: &models.OptionContract{Delta: new(-0.20)}},
		{strike: 98, contract: &models.OptionContract{Delta: new(-0.15)}},
		{strike: 100, contract: &models.OptionContract{Delta: new(-0.10)}},
		{strike: 105, contract: &models.OptionContract{}},
	}

	assert.Equal(t, rows, filterRowsByDelta(rows, &optionChainOpts{}))

	filtered := filterRowsByDelta(rows, &optionChainOpts{
		DeltaMin:    -0.25,
		DeltaMax:    -0.15,
		deltaMinSet: true,
		deltaMaxSet: true,
	})
	require.Len(t, filtered, 3)
	assert.Equal(t, []float64{92, 95, 98}, []float64{filtered[0].strike, filtered[1].strike, filtered[2].strike})

	zeroFiltered := filterRowsByDelta([]chainRow{
		{strike: 100, contract: &models.OptionContract{Delta: new(0.0)}},
		{strike: 105, contract: &models.OptionContract{Delta: new(0.10)}},
	}, &optionChainOpts{DeltaMax: 0, deltaMaxSet: true})
	require.Len(t, zeroFiltered, 1)
	assert.InEpsilon(t, 100.0, zeroFiltered[0].strike, 0.0001)
}

func TestPorcelainChainParams(t *testing.T) {
	params := porcelainChainParams("AAPL", &optionChainOpts{
		Type:        chainContractTypeCall,
		Expiration:  "2026-06-19",
		Strike:      200,
		StrikeRange: strikeRangeNTM,
	})

	assert.Equal(t, "AAPL", params.Symbol)
	assert.Equal(t, marketdata.OptionChainContractType(chainContractTypeCall), params.ContractType)
	assert.Equal(t, marketdata.OptionChainStrategySingle, params.Strategy)
	assert.True(t, params.IncludeUnderlyingQuote)
	assert.Equal(t, "2026-06-19", params.FromDate)
	assert.Equal(t, "2026-06-19", params.ToDate)
	assert.InEpsilon(t, 200.0, params.Strike, 0.0001)
	assert.Equal(t, marketdata.OptionChainRange(strikeRangeNTM), params.Range)
}

func TestOptionChainOptsValidationAdditionalConflicts(t *testing.T) {
	tests := []struct {
		name string
		opts optionChainOpts
	}{
		{name: "invalid type", opts: optionChainOpts{Type: chainContractType("BAD")}},
		{name: "dte expiration", opts: optionChainOpts{DTE: 30, Expiration: "2026-06-19"}},
		{
			name: "delta min greater than max",
			opts: optionChainOpts{DeltaMin: 0.5, DeltaMax: 0.2, deltaMinSet: true, deltaMaxSet: true},
		},
		{name: "negative count", opts: optionChainOpts{StrikeCount: -1}},
		{name: "count min", opts: optionChainOpts{StrikeCount: 2, StrikeMin: 100}},
		{name: "strike combined", opts: optionChainOpts{Strike: 100, StrikeRange: strikeRangeNTM}},
		{name: "count range", opts: optionChainOpts{StrikeCount: 2, StrikeRange: strikeRangeNTM}},
		{name: "min max range", opts: optionChainOpts{StrikeMin: 100, StrikeRange: strikeRangeNTM}},
		{name: "min greater than max", opts: optionChainOpts{StrikeMin: 110, StrikeMax: 100}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.opts.Validate(context.Background())

			require.NotEmpty(t, errs)
		})
	}

	assert.Empty(t, (&optionChainOpts{Type: chainContractTypeAll}).Validate(context.Background()))
}

func TestOptionContractOptsValidationAdditionalConflicts(t *testing.T) {
	tests := []struct {
		name string
		opts optionContractOpts
	}{
		{name: "missing everything", opts: optionContractOpts{}},
		{name: "missing strike", opts: optionContractOpts{Expiration: "2026-06-19", Call: true}},
		{
			name: "both call and put",
			opts: optionContractOpts{Expiration: "2026-06-19", Strike: 200, Call: true, Put: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.opts.Validate(context.Background())

			require.NotEmpty(t, errs)
		})
	}

	validOpts := &optionContractOpts{Expiration: "2026-06-19", Strike: 200, Call: true}
	assert.Empty(t, validOpts.Validate(context.Background()))
}

func fullOptionContractForTest() *models.OptionContract {
	symbol := "AAPL 260619C00200000"
	putCall := "CALL"
	strike := 200.0
	bid := 1.1
	ask := 1.2
	mark := 1.15
	last := 1.05
	delta := 0.5
	gamma := 0.1
	theta := -0.2
	vega := 0.3
	rho := 0.4
	iv := 25.0
	daysToExpiration := 42
	openInterest := int64(100)
	volume := int64(25)
	inTheMoney := true

	return &models.OptionContract{
		Symbol:           &symbol,
		PutCall:          &putCall,
		StrikePrice:      &strike,
		Bid:              &bid,
		Ask:              &ask,
		Mark:             &mark,
		Last:             &last,
		Delta:            &delta,
		Gamma:            &gamma,
		Theta:            &theta,
		Vega:             &vega,
		Rho:              &rho,
		Volatility:       &iv,
		DaysToExpiration: &daysToExpiration,
		OpenInterest:     &openInterest,
		TotalVolume:      &volume,
		InTheMoney:       &inTheMoney,
	}
}
