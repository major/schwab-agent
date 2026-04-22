package commands

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	"github.com/major/schwab-agent/internal/client"
	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/models"
	"github.com/major/schwab-agent/internal/orderbuilder"
	"github.com/major/schwab-agent/internal/output"
)

// testFutureExpTime is a future expiration date for option tests. Computed once
// at package init so all tests use a consistent value. Using AddDate(0, 1, 0)
// ensures the date stays valid regardless of when the test suite runs.
var testFutureExpTime = time.Now().UTC().AddDate(0, 1, 0)

// testFutureExpDate is testFutureExpTime formatted as YYYY-MM-DD for CLI flags.
var testFutureExpDate = testFutureExpTime.Format("2006-01-02")

// testConfigJSON returns a valid config payload for command tests.
// The mutable flag defaults to false (mutable operations disabled).
func testConfigJSON(defaultAccount string) string {
	return `{"client_id":"test-client","client_secret":"test-secret","callback_url":"https://127.0.0.1:8182","default_account":"` + defaultAccount + `"}`
}

// testConfigJSONMutable returns a valid config payload with mutable operations enabled.
func testConfigJSONMutable(defaultAccount string) string {
	return `{"client_id":"test-client","client_secret":"test-secret","callback_url":"https://127.0.0.1:8182","default_account":"` + defaultAccount + `","i-also-like-to-live-dangerously":true}`
}

// writeTestConfig creates a config file for command tests (mutable operations disabled).
func writeTestConfig(t *testing.T, defaultAccount string) string {
	t.Helper()

	configPath := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(configPath, []byte(testConfigJSON(defaultAccount)), 0o600))

	return configPath
}

// writeTestConfigMutable creates a config file with mutable operations enabled.
func writeTestConfigMutable(t *testing.T, defaultAccount string) string {
	t.Helper()

	configPath := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(configPath, []byte(testConfigJSONMutable(defaultAccount)), 0o600))

	return configPath
}

// runOrderCommand runs the order command tree with the provided args and stdin.
func runOrderCommand(t *testing.T, c *client.Ref, configPath, stdin string, args ...string) (string, error) {
	t.Helper()

	var stdout strings.Builder
	root := &cli.Command{
		Name:           "schwab-agent",
		Writer:         &stdout,
		ErrWriter:      io.Discard,
		Reader:         strings.NewReader(stdin),
		ExitErrHandler: func(_ context.Context, _ *cli.Command, _ error) {},
		Commands: []*cli.Command{
			OrderCommand(c, configPath, &stdout),
		},
	}

	fullArgs := append([]string{"schwab-agent"}, args...)
	err := root.Run(context.Background(), fullArgs)

	return stdout.String(), err
}

// decodeEnvelope decodes a success response envelope from the command output.
func decodeEnvelope(t *testing.T, raw string) output.Envelope {
	t.Helper()

	var envelope output.Envelope
	require.NoError(t, json.Unmarshal([]byte(raw), &envelope))

	return envelope
}

// decodeOrderRequest decodes a raw order request JSON payload.
func decodeOrderRequest(t *testing.T, raw string) models.OrderRequest {
	t.Helper()

	var order models.OrderRequest
	require.NoError(t, json.Unmarshal([]byte(raw), &order))

	return order
}

// mustMarshalJSON marshals a value to JSON for test fixtures.
func mustMarshalJSON(t *testing.T, value any) string {
	t.Helper()

	encoded, err := json.Marshal(value)
	require.NoError(t, err)

	return string(encoded)
}

func TestOrderConfirmGate(t *testing.T) {
	t.Parallel()

	configPath := writeTestConfigMutable(t, "hash123")

	testCases := []struct {
		name string
		args []string
	}{
		{
			name: "place equity requires confirm",
			args: []string{"order", "place", "equity", "--symbol", "AAPL", "--action", "BUY", "--quantity", "10"},
		},
		{
			name: "place spec requires confirm",
			args: []string{"order", "place", "--spec", `{"session":"NORMAL","duration":"DAY","orderType":"MARKET","orderStrategyType":"SINGLE","orderLegCollection":[{"instruction":"BUY","quantity":1,"instrument":{"assetType":"EQUITY","symbol":"AAPL"}}]}`},
		},
		{
			name: "cancel requires confirm",
			args: []string{"order", "cancel", "12345"},
		},
		{
			name: "replace requires confirm",
			args: []string{"order", "replace", "12345", "--symbol", "AAPL", "--action", "BUY", "--quantity", "5"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			stdout, err := runOrderCommand(t, nil, configPath, "", testCase.args...)
			require.Error(t, err)
			assert.Empty(t, stdout)

			var validationErr *apperr.ValidationError
			require.ErrorAs(t, err, &validationErr)
			assert.Equal(t, "Add --confirm to execute this order", validationErr.Error())
		})
	}
}

func TestOrderPreviewSpecModes(t *testing.T) {
	t.Parallel()

	previewResponse := models.PreviewOrder{OrderID: ptr(int64(4242))}
	var requestBodies []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/trader/v1/accounts/hash123/previewOrder", r.URL.Path)

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		requestBodies = append(requestBodies, string(body))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(previewResponse))
	}))
	defer server.Close()

	configPath := writeTestConfig(t, "hash123")
	cliClient := &client.Ref{Client: client.NewClient("test-token", client.WithBaseURL(server.URL))}
	spec := mustMarshalJSON(t, &models.OrderRequest{
		Session:           models.SessionNormal,
		Duration:          models.DurationDay,
		OrderType:         models.OrderTypeMarket,
		OrderStrategyType: models.OrderStrategyTypeSingle,
		OrderLegCollection: []models.OrderLegCollection{{
			Instruction: models.InstructionBuy,
			Quantity:    1,
			Instrument: models.OrderInstrument{
				AssetType: models.AssetTypeEquity,
				Symbol:    "AAPL",
			},
		}},
	})
	specFile := filepath.Join(t.TempDir(), "order.json")
	require.NoError(t, os.WriteFile(specFile, []byte(spec), 0o600))

	testCases := []struct {
		name  string
		stdin string
		args  []string
	}{
		{
			name: "inline json",
			args: []string{"order", "preview", "--spec", spec},
		},
		{
			name: "file json",
			args: []string{"order", "preview", "--spec", "@" + specFile},
		},
		{
			name:  "stdin json",
			stdin: spec,
			args:  []string{"order", "preview", "--spec", "-"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			stdout, err := runOrderCommand(t, cliClient, configPath, testCase.stdin, testCase.args...)
			require.NoError(t, err)

			envelope := decodeEnvelope(t, stdout)
			data, ok := envelope.Data.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, float64(4242), data["orderId"])
		})
	}

	require.Len(t, requestBodies, 3)
	for _, body := range requestBodies {
		assert.JSONEq(t, spec, body)
	}
}

func TestOrderBuildEquityOutputsRequestJSON(t *testing.T) {
	t.Parallel()

	stdout, err := runOrderCommand(
		t,
		nil,
		writeTestConfig(t, "hash123"),
		"",
		"order", "build", "equity",
		"--symbol", "AAPL",
		"--action", "BUY",
		"--quantity", "10",
		"--type", "LIMIT",
		"--price", "185.25",
		"--duration", "GOOD_TILL_CANCEL",
		"--session", "AM",
	)
	require.NoError(t, err)

	order := decodeOrderRequest(t, stdout)
	assert.Equal(t, models.SessionAM, order.Session)
	assert.Equal(t, models.DurationGoodTillCancel, order.Duration)
	assert.Equal(t, models.OrderTypeLimit, order.OrderType)
	require.NotNil(t, order.Price)
	assert.Equal(t, 185.25, *order.Price)
	require.Len(t, order.OrderLegCollection, 1)
	assert.Equal(t, "AAPL", order.OrderLegCollection[0].Instrument.Symbol)
	assert.Equal(t, models.InstructionBuy, order.OrderLegCollection[0].Instruction)
}

func TestOrderPlaceEquityPipeline(t *testing.T) {
	t.Parallel()

	var received models.OrderRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/trader/v1/accounts/default-hash/orders", r.URL.Path)

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &received))

		w.Header().Set("Location", "/trader/v1/accounts/default-hash/orders/67890")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	configPath := writeTestConfigMutable(t, "default-hash")
	cliClient := &client.Ref{Client: client.NewClient("test-token", client.WithBaseURL(server.URL))}

	stdout, err := runOrderCommand(
		t,
		cliClient,
		configPath,
		"",
		"order", "place", "equity",
		"--symbol", "AAPL",
		"--action", "BUY",
		"--quantity", "10",
		"--type", "LIMIT",
		"--price", "185.25",
		"--duration", "DAY",
		"--session", "NORMAL",
		"--confirm",
	)
	require.NoError(t, err)

	assert.Equal(t, models.OrderTypeLimit, received.OrderType)
	require.NotNil(t, received.Price)
	assert.Equal(t, 185.25, *received.Price)
	require.Len(t, received.OrderLegCollection, 1)
	assert.Equal(t, "AAPL", received.OrderLegCollection[0].Instrument.Symbol)
	assert.Equal(t, models.InstructionBuy, received.OrderLegCollection[0].Instruction)

	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(67890), data["orderId"])
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
}

func TestOrderPlaceSpecFromFile(t *testing.T) {
	t.Parallel()

	var received models.OrderRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/trader/v1/accounts/hash123/orders", r.URL.Path)

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &received))

		w.Header().Set("Location", "/trader/v1/accounts/hash123/orders/24680")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	configPath := writeTestConfigMutable(t, "hash123")
	cliClient := &client.Ref{Client: client.NewClient("test-token", client.WithBaseURL(server.URL))}
	orderRequest, err := orderbuilder.BuildEquityOrder(&orderbuilder.EquityParams{
		Symbol:    "MSFT",
		Action:    models.InstructionBuy,
		Quantity:  2,
		OrderType: models.OrderTypeMarket,
	})
	require.NoError(t, err)

	specPath := filepath.Join(t.TempDir(), "spec.json")
	require.NoError(t, os.WriteFile(specPath, []byte(mustMarshalJSON(t, orderRequest)), 0o600))

	stdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "place", "--spec", "@"+specPath, "--confirm")
	require.NoError(t, err)

	require.Len(t, received.OrderLegCollection, 1)
	assert.Equal(t, "MSFT", received.OrderLegCollection[0].Instrument.Symbol)
	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(24680), data["orderId"])
}

func TestOrderMutableGuard(t *testing.T) {
	t.Parallel()

	// Config WITHOUT i-also-like-to-live-dangerously (default: false).
	configPath := writeTestConfig(t, "hash123")

	testCases := []struct {
		name string
		args []string
	}{
		{
			name: "place spec blocked without mutable flag",
			args: []string{"order", "place", "--spec", `{"session":"NORMAL","duration":"DAY","orderType":"MARKET","orderStrategyType":"SINGLE","orderLegCollection":[{"instruction":"BUY","quantity":1,"instrument":{"assetType":"EQUITY","symbol":"AAPL"}}]}`, "--confirm"},
		},
		{
			name: "place equity blocked without mutable flag",
			args: []string{"order", "place", "equity", "--symbol", "AAPL", "--action", "BUY", "--quantity", "10", "--confirm"},
		},
		{
			name: "cancel blocked without mutable flag",
			args: []string{"order", "cancel", "12345", "--confirm"},
		},
		{
			name: "replace blocked without mutable flag",
			args: []string{"order", "replace", "12345", "--symbol", "AAPL", "--action", "BUY", "--quantity", "5", "--confirm"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			stdout, err := runOrderCommand(t, nil, configPath, "", testCase.args...)
			require.Error(t, err)
			assert.Empty(t, stdout)

			var validationErr *apperr.ValidationError
			require.ErrorAs(t, err, &validationErr)
			assert.Contains(t, validationErr.Error(), "i-also-like-to-live-dangerously")
		})
	}
}

func TestOrderMutableGuardTakesPriorityOverConfirm(t *testing.T) {
	t.Parallel()

	// Config WITHOUT mutable flag, command WITHOUT --confirm.
	// The mutable guard should fire first.
	configPath := writeTestConfig(t, "hash123")

	stdout, err := runOrderCommand(t, nil, configPath, "",
		"order", "place", "equity",
		"--symbol", "AAPL", "--action", "BUY", "--quantity", "10",
	)
	require.Error(t, err)
	assert.Empty(t, stdout)

	var validationErr *apperr.ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Contains(t, validationErr.Error(), "i-also-like-to-live-dangerously")
}

func TestOrderPreviewNotBlockedByMutableGuard(t *testing.T) {
	t.Parallel()

	previewResponse := models.PreviewOrder{OrderID: ptr(int64(9999))}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(previewResponse))
	}))
	defer server.Close()

	// Config WITHOUT mutable flag - preview should still work.
	configPath := writeTestConfig(t, "hash123")
	cliClient := &client.Ref{Client: client.NewClient("test-token", client.WithBaseURL(server.URL))}

	spec := `{"session":"NORMAL","duration":"DAY","orderType":"MARKET","orderStrategyType":"SINGLE","orderLegCollection":[{"instruction":"BUY","quantity":1,"instrument":{"assetType":"EQUITY","symbol":"AAPL"}}]}`
	stdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "preview", "--spec", spec)
	require.NoError(t, err)
	assert.NotEmpty(t, stdout)
}

func TestOrderBuildOCOOutputsRequestJSON(t *testing.T) {
	t.Parallel()

	stdout, err := runOrderCommand(
		t,
		nil,
		writeTestConfig(t, "hash123"),
		"",
		"order", "build", "oco",
		"--symbol", "AAPL",
		"--action", "SELL",
		"--quantity", "100",
		"--take-profit", "160",
		"--stop-loss", "140",
		"--duration", "GOOD_TILL_CANCEL",
		"--session", "NORMAL",
	)
	require.NoError(t, err)

	order := decodeOrderRequest(t, stdout)
	assert.Equal(t, models.OrderStrategyTypeOCO, order.OrderStrategyType)
	require.Len(t, order.ChildOrderStrategies, 2)

	takeProfit := order.ChildOrderStrategies[0]
	assert.Equal(t, models.OrderTypeLimit, takeProfit.OrderType)
	require.NotNil(t, takeProfit.Price)
	assert.Equal(t, 160.0, *takeProfit.Price)
	assert.Equal(t, models.InstructionSell, takeProfit.OrderLegCollection[0].Instruction)

	stopLoss := order.ChildOrderStrategies[1]
	assert.Equal(t, models.OrderTypeStop, stopLoss.OrderType)
	require.NotNil(t, stopLoss.StopPrice)
	assert.Equal(t, 140.0, *stopLoss.StopPrice)
}

func TestOrderBuildOCOSingleExit(t *testing.T) {
	t.Parallel()

	// Stop-loss only: should produce a plain SINGLE order, not an OCO wrapper.
	stdout, err := runOrderCommand(
		t,
		nil,
		writeTestConfig(t, "hash123"),
		"",
		"order", "build", "oco",
		"--symbol", "AAPL",
		"--action", "SELL",
		"--quantity", "50",
		"--stop-loss", "140",
	)
	require.NoError(t, err)

	order := decodeOrderRequest(t, stdout)
	assert.Equal(t, models.OrderStrategyTypeSingle, order.OrderStrategyType)
	assert.Equal(t, models.OrderTypeStop, order.OrderType)
	require.NotNil(t, order.StopPrice)
	assert.Equal(t, 140.0, *order.StopPrice)
	assert.Empty(t, order.ChildOrderStrategies)
}

func TestOrderPlaceOCOPipeline(t *testing.T) {
	t.Parallel()

	var received models.OrderRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/trader/v1/accounts/default-hash/orders", r.URL.Path)

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &received))

		w.Header().Set("Location", "/trader/v1/accounts/default-hash/orders/55555")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	configPath := writeTestConfigMutable(t, "default-hash")
	cliClient := &client.Ref{Client: client.NewClient("test-token", client.WithBaseURL(server.URL))}

	stdout, err := runOrderCommand(
		t,
		cliClient,
		configPath,
		"",
		"order", "place", "oco",
		"--symbol", "AAPL",
		"--action", "SELL",
		"--quantity", "100",
		"--take-profit", "160",
		"--stop-loss", "140",
		"--confirm",
	)
	require.NoError(t, err)

	// Verify the OCO structure was sent to the API.
	assert.Equal(t, models.OrderStrategyTypeOCO, received.OrderStrategyType)
	require.Len(t, received.ChildOrderStrategies, 2)

	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(55555), data["orderId"])
}

// ptr returns a pointer to the provided value (generic test helper).
func ptr[T any](v T) *T {
	return &v
}

// --- Spread build tests (iron condor, vertical, strangle, straddle, covered call) ---
//
// These tests exercise the full CLI pipeline: CLI flags → parseXxxParams → validate → build → JSON.
// The builder functions are independently tested in orderbuilder/spread_test.go; these tests verify
// the parsing layer wires flags to params correctly, which is the untested gap that matters most
// for order correctness.

func TestOrderBuildIronCondorOpen(t *testing.T) {
	t.Parallel()

	stdout, err := runOrderCommand(
		t, nil, writeTestConfig(t, "hash123"), "",
		"order", "build", "iron-condor",
		"--underlying", "SPY",
		"--expiration", testFutureExpDate,
		"--put-long-strike", "400",
		"--put-short-strike", "410",
		"--call-short-strike", "420",
		"--call-long-strike", "430",
		"--open",
		"--quantity", "5",
		"--price", "2.50",
		"--duration", "DAY",
		"--session", "NORMAL",
	)
	require.NoError(t, err)

	order := decodeOrderRequest(t, stdout)

	// Order-level fields.
	assert.Equal(t, models.OrderTypeNetCredit, order.OrderType)
	assert.Equal(t, models.OrderStrategyTypeSingle, order.OrderStrategyType)
	require.NotNil(t, order.ComplexOrderStrategyType)
	assert.Equal(t, models.ComplexOrderStrategyTypeIronCondor, *order.ComplexOrderStrategyType)
	require.NotNil(t, order.Price)
	assert.Equal(t, 2.50, *order.Price)
	assert.Equal(t, models.DurationDay, order.Duration)
	assert.Equal(t, models.SessionNormal, order.Session)

	// 4 legs: put-long, put-short, call-short, call-long.
	require.Len(t, order.OrderLegCollection, 4)

	assert.Equal(t, models.InstructionBuyToOpen, order.OrderLegCollection[0].Instruction)
	require.NotNil(t, order.OrderLegCollection[0].Instrument.PutCall)
	assert.Equal(t, models.PutCallPut, *order.OrderLegCollection[0].Instrument.PutCall)
	require.NotNil(t, order.OrderLegCollection[0].Instrument.OptionStrikePrice)
	assert.Equal(t, 400.0, *order.OrderLegCollection[0].Instrument.OptionStrikePrice)

	assert.Equal(t, models.InstructionSellToOpen, order.OrderLegCollection[1].Instruction)
	require.NotNil(t, order.OrderLegCollection[1].Instrument.OptionStrikePrice)
	assert.Equal(t, 410.0, *order.OrderLegCollection[1].Instrument.OptionStrikePrice)

	assert.Equal(t, models.InstructionSellToOpen, order.OrderLegCollection[2].Instruction)
	require.NotNil(t, order.OrderLegCollection[2].Instrument.PutCall)
	assert.Equal(t, models.PutCallCall, *order.OrderLegCollection[2].Instrument.PutCall)
	require.NotNil(t, order.OrderLegCollection[2].Instrument.OptionStrikePrice)
	assert.Equal(t, 420.0, *order.OrderLegCollection[2].Instrument.OptionStrikePrice)

	assert.Equal(t, models.InstructionBuyToOpen, order.OrderLegCollection[3].Instruction)
	require.NotNil(t, order.OrderLegCollection[3].Instrument.OptionStrikePrice)
	assert.Equal(t, 430.0, *order.OrderLegCollection[3].Instrument.OptionStrikePrice)

	// All legs should have the same quantity.
	for i, leg := range order.OrderLegCollection {
		assert.Equal(t, 5.0, leg.Quantity, "leg %d quantity", i)
	}
}

func TestOrderBuildIronCondorClose(t *testing.T) {
	t.Parallel()

	stdout, err := runOrderCommand(
		t, nil, writeTestConfig(t, "hash123"), "",
		"order", "build", "iron-condor",
		"--underlying", "SPY",
		"--expiration", testFutureExpDate,
		"--put-long-strike", "400",
		"--put-short-strike", "410",
		"--call-short-strike", "420",
		"--call-long-strike", "430",
		"--close",
		"--quantity", "5",
		"--price", "1.00",
	)
	require.NoError(t, err)

	order := decodeOrderRequest(t, stdout)
	assert.Equal(t, models.OrderTypeNetDebit, order.OrderType)

	// Closing: long legs SELL_TO_CLOSE, short legs BUY_TO_CLOSE.
	require.Len(t, order.OrderLegCollection, 4)
	assert.Equal(t, models.InstructionSellToClose, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, models.InstructionBuyToClose, order.OrderLegCollection[1].Instruction)
	assert.Equal(t, models.InstructionBuyToClose, order.OrderLegCollection[2].Instruction)
	assert.Equal(t, models.InstructionSellToClose, order.OrderLegCollection[3].Instruction)
}

func TestOrderBuildVerticalCallDebit(t *testing.T) {
	t.Parallel()

	// Bull call spread: buy lower strike, sell higher strike = NET_DEBIT.
	stdout, err := runOrderCommand(
		t, nil, writeTestConfig(t, "hash123"), "",
		"order", "build", "vertical",
		"--underlying", "AAPL",
		"--expiration", testFutureExpDate,
		"--long-strike", "180",
		"--short-strike", "190",
		"--call",
		"--open",
		"--quantity", "3",
		"--price", "4.00",
		"--duration", "GOOD_TILL_CANCEL",
		"--session", "AM",
	)
	require.NoError(t, err)

	order := decodeOrderRequest(t, stdout)
	assert.Equal(t, models.OrderTypeNetDebit, order.OrderType)
	require.NotNil(t, order.ComplexOrderStrategyType)
	assert.Equal(t, models.ComplexOrderStrategyTypeVertical, *order.ComplexOrderStrategyType)
	assert.Equal(t, models.DurationGoodTillCancel, order.Duration)
	assert.Equal(t, models.SessionAM, order.Session)
	require.NotNil(t, order.Price)
	assert.Equal(t, 4.00, *order.Price)

	require.Len(t, order.OrderLegCollection, 2)
	assert.Equal(t, models.InstructionBuyToOpen, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, 3.0, order.OrderLegCollection[0].Quantity)
	require.NotNil(t, order.OrderLegCollection[0].Instrument.OptionStrikePrice)
	assert.Equal(t, 180.0, *order.OrderLegCollection[0].Instrument.OptionStrikePrice)

	assert.Equal(t, models.InstructionSellToOpen, order.OrderLegCollection[1].Instruction)
	require.NotNil(t, order.OrderLegCollection[1].Instrument.OptionStrikePrice)
	assert.Equal(t, 190.0, *order.OrderLegCollection[1].Instrument.OptionStrikePrice)
}

func TestOrderBuildVerticalPutCredit(t *testing.T) {
	t.Parallel()

	// Bull put spread: buy lower strike put, sell higher strike put = NET_CREDIT.
	stdout, err := runOrderCommand(
		t, nil, writeTestConfig(t, "hash123"), "",
		"order", "build", "vertical",
		"--underlying", "AAPL",
		"--expiration", testFutureExpDate,
		"--long-strike", "170",
		"--short-strike", "180",
		"--put",
		"--open",
		"--quantity", "2",
		"--price", "3.00",
	)
	require.NoError(t, err)

	order := decodeOrderRequest(t, stdout)
	assert.Equal(t, models.OrderTypeNetCredit, order.OrderType)
	require.NotNil(t, order.ComplexOrderStrategyType)
	assert.Equal(t, models.ComplexOrderStrategyTypeVertical, *order.ComplexOrderStrategyType)
	require.Len(t, order.OrderLegCollection, 2)

	// Both legs should be puts.
	for _, leg := range order.OrderLegCollection {
		require.NotNil(t, leg.Instrument.PutCall)
		assert.Equal(t, models.PutCallPut, *leg.Instrument.PutCall)
	}
}

func TestOrderBuildStrangleBuyOpen(t *testing.T) {
	t.Parallel()

	stdout, err := runOrderCommand(
		t, nil, writeTestConfig(t, "hash123"), "",
		"order", "build", "strangle",
		"--underlying", "TSLA",
		"--expiration", testFutureExpDate,
		"--call-strike", "300",
		"--put-strike", "250",
		"--buy",
		"--open",
		"--quantity", "2",
		"--price", "15.00",
		"--duration", "DAY",
		"--session", "NORMAL",
	)
	require.NoError(t, err)

	order := decodeOrderRequest(t, stdout)
	assert.Equal(t, models.OrderTypeNetDebit, order.OrderType)
	require.NotNil(t, order.ComplexOrderStrategyType)
	assert.Equal(t, models.ComplexOrderStrategyTypeStrangle, *order.ComplexOrderStrategyType)
	require.NotNil(t, order.Price)
	assert.Equal(t, 15.00, *order.Price)

	require.Len(t, order.OrderLegCollection, 2)

	// Leg 0: call at 300.
	assert.Equal(t, models.InstructionBuyToOpen, order.OrderLegCollection[0].Instruction)
	require.NotNil(t, order.OrderLegCollection[0].Instrument.PutCall)
	assert.Equal(t, models.PutCallCall, *order.OrderLegCollection[0].Instrument.PutCall)
	require.NotNil(t, order.OrderLegCollection[0].Instrument.OptionStrikePrice)
	assert.Equal(t, 300.0, *order.OrderLegCollection[0].Instrument.OptionStrikePrice)

	// Leg 1: put at 250.
	assert.Equal(t, models.InstructionBuyToOpen, order.OrderLegCollection[1].Instruction)
	require.NotNil(t, order.OrderLegCollection[1].Instrument.PutCall)
	assert.Equal(t, models.PutCallPut, *order.OrderLegCollection[1].Instrument.PutCall)
	require.NotNil(t, order.OrderLegCollection[1].Instrument.OptionStrikePrice)
	assert.Equal(t, 250.0, *order.OrderLegCollection[1].Instrument.OptionStrikePrice)
}

func TestOrderBuildStrangleSellOpen(t *testing.T) {
	t.Parallel()

	stdout, err := runOrderCommand(
		t, nil, writeTestConfig(t, "hash123"), "",
		"order", "build", "strangle",
		"--underlying", "TSLA",
		"--expiration", testFutureExpDate,
		"--call-strike", "300",
		"--put-strike", "250",
		"--sell",
		"--open",
		"--quantity", "1",
		"--price", "12.00",
	)
	require.NoError(t, err)

	order := decodeOrderRequest(t, stdout)
	assert.Equal(t, models.OrderTypeNetCredit, order.OrderType)
	assert.Equal(t, models.InstructionSellToOpen, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, models.InstructionSellToOpen, order.OrderLegCollection[1].Instruction)
}

func TestOrderBuildStraddleBuyOpen(t *testing.T) {
	t.Parallel()

	stdout, err := runOrderCommand(
		t, nil, writeTestConfig(t, "hash123"), "",
		"order", "build", "straddle",
		"--underlying", "NVDA",
		"--expiration", testFutureExpDate,
		"--strike", "130",
		"--buy",
		"--open",
		"--quantity", "4",
		"--price", "20.00",
		"--duration", "DAY",
		"--session", "NORMAL",
	)
	require.NoError(t, err)

	order := decodeOrderRequest(t, stdout)
	assert.Equal(t, models.OrderTypeNetDebit, order.OrderType)
	require.NotNil(t, order.ComplexOrderStrategyType)
	assert.Equal(t, models.ComplexOrderStrategyTypeStraddle, *order.ComplexOrderStrategyType)
	require.NotNil(t, order.Price)
	assert.Equal(t, 20.00, *order.Price)

	// Both legs at same strike, both BUY_TO_OPEN, one call + one put.
	require.Len(t, order.OrderLegCollection, 2)

	assert.Equal(t, models.InstructionBuyToOpen, order.OrderLegCollection[0].Instruction)
	require.NotNil(t, order.OrderLegCollection[0].Instrument.PutCall)
	assert.Equal(t, models.PutCallCall, *order.OrderLegCollection[0].Instrument.PutCall)
	require.NotNil(t, order.OrderLegCollection[0].Instrument.OptionStrikePrice)
	assert.Equal(t, 130.0, *order.OrderLegCollection[0].Instrument.OptionStrikePrice)

	assert.Equal(t, models.InstructionBuyToOpen, order.OrderLegCollection[1].Instruction)
	require.NotNil(t, order.OrderLegCollection[1].Instrument.PutCall)
	assert.Equal(t, models.PutCallPut, *order.OrderLegCollection[1].Instrument.PutCall)
	require.NotNil(t, order.OrderLegCollection[1].Instrument.OptionStrikePrice)
	assert.Equal(t, 130.0, *order.OrderLegCollection[1].Instrument.OptionStrikePrice)

	for i, leg := range order.OrderLegCollection {
		assert.Equal(t, 4.0, leg.Quantity, "leg %d quantity", i)
	}
}

func TestOrderBuildStraddleSellOpen(t *testing.T) {
	t.Parallel()

	stdout, err := runOrderCommand(
		t, nil, writeTestConfig(t, "hash123"), "",
		"order", "build", "straddle",
		"--underlying", "NVDA",
		"--expiration", testFutureExpDate,
		"--strike", "130",
		"--sell",
		"--open",
		"--quantity", "1",
		"--price", "18.00",
	)
	require.NoError(t, err)

	order := decodeOrderRequest(t, stdout)
	assert.Equal(t, models.OrderTypeNetCredit, order.OrderType)
	assert.Equal(t, models.InstructionSellToOpen, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, models.InstructionSellToOpen, order.OrderLegCollection[1].Instruction)
}

func TestOrderBuildCoveredCallOutputsRequestJSON(t *testing.T) {
	t.Parallel()

	stdout, err := runOrderCommand(
		t, nil, writeTestConfig(t, "hash123"), "",
		"order", "build", "covered-call",
		"--underlying", "F",
		"--expiration", testFutureExpDate,
		"--strike", "14",
		"--quantity", "2",
		"--price", "12.50",
		"--duration", "DAY",
		"--session", "NORMAL",
	)
	require.NoError(t, err)

	order := decodeOrderRequest(t, stdout)
	assert.Equal(t, models.OrderTypeNetDebit, order.OrderType)
	require.NotNil(t, order.ComplexOrderStrategyType)
	assert.Equal(t, models.ComplexOrderStrategyTypeCovered, *order.ComplexOrderStrategyType)
	require.NotNil(t, order.Price)
	assert.Equal(t, 12.50, *order.Price)

	// Two legs: equity BUY + option SELL_TO_OPEN.
	require.Len(t, order.OrderLegCollection, 2)

	// Equity leg: 2 contracts = 200 shares.
	equityLeg := order.OrderLegCollection[0]
	assert.Equal(t, models.InstructionBuy, equityLeg.Instruction)
	assert.Equal(t, 200.0, equityLeg.Quantity)
	assert.Equal(t, models.AssetTypeEquity, equityLeg.Instrument.AssetType)
	assert.Equal(t, "F", equityLeg.Instrument.Symbol)

	// Option leg: SELL_TO_OPEN call.
	optionLeg := order.OrderLegCollection[1]
	assert.Equal(t, models.InstructionSellToOpen, optionLeg.Instruction)
	assert.Equal(t, 2.0, optionLeg.Quantity)
	assert.Equal(t, models.AssetTypeOption, optionLeg.Instrument.AssetType)
	require.NotNil(t, optionLeg.Instrument.PutCall)
	assert.Equal(t, models.PutCallCall, *optionLeg.Instrument.PutCall)
	require.NotNil(t, optionLeg.Instrument.OptionStrikePrice)
	assert.Equal(t, 14.0, *optionLeg.Instrument.OptionStrikePrice)
}

func TestOrderBuildSpreadDefaultDurationSession(t *testing.T) {
	t.Parallel()

	// Omit --duration and --session; defaults should be DAY and NORMAL.
	testCases := []struct {
		name string
		args []string
	}{
		{
			name: "iron-condor defaults",
			args: []string{
				"order", "build", "iron-condor",
				"--underlying", "SPY", "--expiration", testFutureExpDate,
				"--put-long-strike", "400", "--put-short-strike", "410",
				"--call-short-strike", "420", "--call-long-strike", "430",
				"--open", "--quantity", "1", "--price", "1.00",
			},
		},
		{
			name: "vertical defaults",
			args: []string{
				"order", "build", "vertical",
				"--underlying", "AAPL", "--expiration", testFutureExpDate,
				"--long-strike", "180", "--short-strike", "190",
				"--call", "--open", "--quantity", "1", "--price", "2.00",
			},
		},
		{
			name: "strangle defaults",
			args: []string{
				"order", "build", "strangle",
				"--underlying", "TSLA", "--expiration", testFutureExpDate,
				"--call-strike", "300", "--put-strike", "250",
				"--buy", "--open", "--quantity", "1", "--price", "10.00",
			},
		},
		{
			name: "straddle defaults",
			args: []string{
				"order", "build", "straddle",
				"--underlying", "NVDA", "--expiration", testFutureExpDate,
				"--strike", "130",
				"--buy", "--open", "--quantity", "1", "--price", "15.00",
			},
		},
		{
			name: "covered-call defaults",
			args: []string{
				"order", "build", "covered-call",
				"--underlying", "F", "--expiration", testFutureExpDate,
				"--strike", "14", "--quantity", "1", "--price", "12.00",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stdout, err := runOrderCommand(t, nil, writeTestConfig(t, "hash123"), "", tc.args...)
			require.NoError(t, err)

			order := decodeOrderRequest(t, stdout)
			assert.Equal(t, models.DurationDay, order.Duration, "default duration should be DAY")
			assert.Equal(t, models.SessionNormal, order.Session, "default session should be NORMAL")
		})
	}
}

// TestOrderBuildParseErrors covers error paths in the param parser functions
// (parseEquityParams, parseOptionParams, parseBracketParams, parseOCOParams)
// that aren't exercised by the spread tests.
func TestOrderBuildParseErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		args    []string
		wantMsg string
	}{
		// parseInstruction invalid value (exercises default branch).
		{
			name: "equity invalid action",
			args: []string{
				"order", "build", "equity",
				"--symbol", "AAPL", "--action", "YOLO",
				"--quantity", "10",
			},
			wantMsg: "action is invalid",
		},
		{
			name: "option invalid action",
			args: []string{
				"order", "build", "option",
				"--underlying", "AAPL", "--action", "YOLO",
				"--call", "--expiration", "2026-06-19",
				"--strike", "150", "--quantity", "1",
			},
			wantMsg: "action is invalid",
		},
		{
			name: "bracket invalid action",
			args: []string{
				"order", "build", "bracket",
				"--symbol", "AAPL", "--action", "YOLO",
				"--quantity", "10", "--take-profit", "200",
			},
			wantMsg: "action is invalid",
		},
		{
			name: "oco invalid action",
			args: []string{
				"order", "build", "oco",
				"--symbol", "AAPL", "--action", "YOLO",
				"--quantity", "10", "--stop-loss", "140",
			},
			wantMsg: "action is invalid",
		},
		// parseOrderType invalid value (exercises default branch).
		{
			name: "equity invalid order type",
			args: []string{
				"order", "build", "equity",
				"--symbol", "AAPL", "--action", "BUY",
				"--quantity", "10", "--type", "BANANAS",
			},
			wantMsg: "type is invalid",
		},
		// parseDurationSession error in OCO path.
		{
			name: "oco invalid duration",
			args: []string{
				"order", "build", "oco",
				"--symbol", "AAPL", "--action", "SELL",
				"--quantity", "10", "--stop-loss", "140",
				"--duration", "ETERNITY",
			},
			wantMsg: "duration is invalid",
		},
		{
			name: "oco invalid session",
			args: []string{
				"order", "build", "oco",
				"--symbol", "AAPL", "--action", "SELL",
				"--quantity", "10", "--stop-loss", "140",
				"--session", "MIDNIGHT",
			},
			wantMsg: "session is invalid",
		},
		// parseDurationSession error in equity path.
		{
			name: "equity invalid duration",
			args: []string{
				"order", "build", "equity",
				"--symbol", "AAPL", "--action", "BUY",
				"--quantity", "10", "--duration", "FOREVER",
			},
			wantMsg: "duration is invalid",
		},
		// parseDurationSession error in bracket path.
		{
			name: "bracket invalid duration",
			args: []string{
				"order", "build", "bracket",
				"--symbol", "AAPL", "--action", "BUY",
				"--quantity", "10", "--take-profit", "200",
				"--duration", "FOREVER",
			},
			wantMsg: "duration is invalid",
		},
		// parseExpiration error in option path (put/call valid but expiration bad).
		{
			name: "option bad expiration",
			args: []string{
				"order", "build", "option",
				"--underlying", "AAPL", "--action", "BUY_TO_OPEN",
				"--call", "--expiration", "not-a-date",
				"--strike", "150", "--quantity", "1",
			},
			wantMsg: "expiration must use YYYY-MM-DD format",
		},
		// parsePutCall error in option path.
		{
			name: "option missing put/call",
			args: []string{
				"order", "build", "option",
				"--underlying", "AAPL", "--action", "BUY_TO_OPEN",
				"--expiration", "2026-06-19",
				"--strike", "150", "--quantity", "1",
			},
			wantMsg: "exactly one of --call or --put is required",
		},
		// parseDurationSession error in option path.
		{
			name: "option invalid session",
			args: []string{
				"order", "build", "option",
				"--underlying", "AAPL", "--action", "BUY_TO_OPEN",
				"--call", "--expiration", "2026-06-19",
				"--strike", "150", "--quantity", "1",
				"--session", "MIDNIGHT",
			},
			wantMsg: "session is invalid",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stdout, err := runOrderCommand(t, nil, writeTestConfig(t, "hash123"), "", tc.args...)
			require.Error(t, err)
			assert.Empty(t, stdout)

			var validationErr *apperr.ValidationError
			require.ErrorAs(t, err, &validationErr)
			assert.Equal(t, tc.wantMsg, validationErr.Error())
		})
	}
}

func TestOrderBuildSpreadParseErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		args    []string
		wantMsg string
	}{
		// Invalid expiration format.
		{
			name: "iron-condor bad expiration",
			args: []string{
				"order", "build", "iron-condor",
				"--underlying", "SPY", "--expiration", "June-19-2026",
				"--put-long-strike", "400", "--put-short-strike", "410",
				"--call-short-strike", "420", "--call-long-strike", "430",
				"--open", "--quantity", "1", "--price", "1.00",
			},
			wantMsg: "expiration must use YYYY-MM-DD format",
		},
		{
			name: "vertical bad expiration",
			args: []string{
				"order", "build", "vertical",
				"--underlying", "AAPL", "--expiration", "not-a-date",
				"--long-strike", "180", "--short-strike", "190",
				"--call", "--open", "--quantity", "1", "--price", "2.00",
			},
			wantMsg: "expiration must use YYYY-MM-DD format",
		},
		{
			name: "covered-call bad expiration",
			args: []string{
				"order", "build", "covered-call",
				"--underlying", "F", "--expiration", "2026/06/19",
				"--strike", "14", "--quantity", "1", "--price", "12.00",
			},
			wantMsg: "expiration must use YYYY-MM-DD format",
		},
		// Missing mutually exclusive open/close.
		{
			name: "iron-condor missing open/close",
			args: []string{
				"order", "build", "iron-condor",
				"--underlying", "SPY", "--expiration", testFutureExpDate,
				"--put-long-strike", "400", "--put-short-strike", "410",
				"--call-short-strike", "420", "--call-long-strike", "430",
				"--quantity", "1", "--price", "1.00",
			},
			wantMsg: "exactly one of --open or --close is required",
		},
		{
			name: "vertical missing open/close",
			args: []string{
				"order", "build", "vertical",
				"--underlying", "AAPL", "--expiration", testFutureExpDate,
				"--long-strike", "180", "--short-strike", "190",
				"--call", "--quantity", "1", "--price", "2.00",
			},
			wantMsg: "exactly one of --open or --close is required",
		},
		// Missing mutually exclusive buy/sell.
		{
			name: "strangle missing buy/sell",
			args: []string{
				"order", "build", "strangle",
				"--underlying", "TSLA", "--expiration", testFutureExpDate,
				"--call-strike", "300", "--put-strike", "250",
				"--open", "--quantity", "1", "--price", "10.00",
			},
			wantMsg: "exactly one of --buy or --sell is required",
		},
		{
			name: "straddle missing buy/sell",
			args: []string{
				"order", "build", "straddle",
				"--underlying", "NVDA", "--expiration", testFutureExpDate,
				"--strike", "130",
				"--open", "--quantity", "1", "--price", "15.00",
			},
			wantMsg: "exactly one of --buy or --sell is required",
		},
		// Missing mutually exclusive call/put.
		{
			name: "vertical missing call/put",
			args: []string{
				"order", "build", "vertical",
				"--underlying", "AAPL", "--expiration", testFutureExpDate,
				"--long-strike", "180", "--short-strike", "190",
				"--open", "--quantity", "1", "--price", "2.00",
			},
			wantMsg: "exactly one of --call or --put is required",
		},
		// Invalid duration.
		{
			name: "strangle invalid duration",
			args: []string{
				"order", "build", "strangle",
				"--underlying", "TSLA", "--expiration", testFutureExpDate,
				"--call-strike", "300", "--put-strike", "250",
				"--buy", "--open", "--quantity", "1", "--price", "10.00",
				"--duration", "FOREVER",
			},
			wantMsg: "duration is invalid",
		},
		// Invalid session.
		{
			name: "straddle invalid session",
			args: []string{
				"order", "build", "straddle",
				"--underlying", "NVDA", "--expiration", testFutureExpDate,
				"--strike", "130",
				"--buy", "--open", "--quantity", "1", "--price", "15.00",
				"--session", "AFTERHOURS",
			},
			wantMsg: "session is invalid",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stdout, err := runOrderCommand(t, nil, writeTestConfig(t, "hash123"), "", tc.args...)
			require.Error(t, err)
			assert.Empty(t, stdout)

			var validationErr *apperr.ValidationError
			require.ErrorAs(t, err, &validationErr)
			assert.Equal(t, tc.wantMsg, validationErr.Error())
		})
	}
}

func TestOrderListAllAccounts(t *testing.T) {
	t.Parallel()

	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/trader/v1/orders", r.URL.Path)
		assert.NotEmpty(t, r.URL.Query().Get("fromEnteredTime"))
		assert.NotEmpty(t, r.URL.Query().Get("toEnteredTime"))

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`[{"session":"NORMAL","duration":"DAY","orderType":"MARKET","orderStrategyType":"SINGLE","orderId":12345,"status":"FILLED","orderLegCollection":[{"instruction":"BUY","quantity":10,"instrument":{"assetType":"EQUITY","symbol":"AAPL"}}]}]`))
		require.NoError(t, err)
	}))
	defer server.Close()

	configPath := writeTestConfig(t, "hash123")
	cliClient := testClient(t, server)

	// Act
	stdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "list")

	// Assert
	require.NoError(t, err)
	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	orders, ok := data["orders"].([]any)
	require.True(t, ok)
	require.Len(t, orders, 1)
	order, ok := orders[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(12345), order["orderId"])
	assert.Equal(t, "FILLED", order["status"])
}

func TestOrderListWithAccount(t *testing.T) {
	t.Parallel()

	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/trader/v1/accounts/hash123/orders", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`[{"session":"NORMAL","duration":"DAY","orderType":"MARKET","orderStrategyType":"SINGLE","orderId":54321,"status":"QUEUED","orderLegCollection":[{"instruction":"SELL","quantity":5,"instrument":{"assetType":"EQUITY","symbol":"MSFT"}}]}]`))
		require.NoError(t, err)
	}))
	defer server.Close()

	configPath := writeTestConfig(t, "unused-default")
	cliClient := testClient(t, server)

	// Act
	stdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "list", "--account", "hash123")

	// Assert
	require.NoError(t, err)
	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	orders, ok := data["orders"].([]any)
	require.True(t, ok)
	require.Len(t, orders, 1)
	order, ok := orders[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(54321), order["orderId"])
	assert.Equal(t, "QUEUED", order["status"])
}

func TestOrderListWithFilters(t *testing.T) {
	t.Parallel()

	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/trader/v1/orders", r.URL.Path)
		assert.Equal(t, "FILLED", r.URL.Query().Get("status"))
		assert.Equal(t, "2025-01-01T00:00:00Z", r.URL.Query().Get("fromEnteredTime"))
		assert.Equal(t, "2025-12-31T00:00:00Z", r.URL.Query().Get("toEnteredTime"))

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`[]`))
		require.NoError(t, err)
	}))
	defer server.Close()

	configPath := writeTestConfig(t, "hash123")
	cliClient := testClient(t, server)

	// Act
	stdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "list", "--status", "FILLED", "--from", "2025-01-01T00:00:00Z", "--to", "2025-12-31T00:00:00Z")

	// Assert
	require.NoError(t, err)
	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	orders, ok := data["orders"].([]any)
	require.True(t, ok)
	assert.Empty(t, orders)
}

func TestOrderListAPIError(t *testing.T) {
	t.Parallel()

	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/trader/v1/orders", r.URL.Path)
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	configPath := writeTestConfig(t, "hash123")
	cliClient := testClient(t, server)

	// Act
	stdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "list")

	// Assert
	require.Error(t, err)
	assert.Empty(t, stdout)
	var httpErr *apperr.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusInternalServerError, httpErr.StatusCode)
}

func TestOrderGetSuccess(t *testing.T) {
	t.Parallel()

	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/trader/v1/accounts/hash123/orders/12345", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"session":"NORMAL","duration":"DAY","orderType":"MARKET","orderStrategyType":"SINGLE","orderId":12345,"status":"FILLED","orderLegCollection":[{"instruction":"BUY","quantity":10,"instrument":{"assetType":"EQUITY","symbol":"AAPL"}}]}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	configPath := writeTestConfig(t, "hash123")
	cliClient := testClient(t, server)

	// Act
	stdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "get", "12345")

	// Assert
	require.NoError(t, err)
	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	order, ok := data["order"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(12345), order["orderId"])
	assert.Equal(t, "FILLED", order["status"])
}

func TestOrderGetNoAccount(t *testing.T) {
	t.Parallel()

	// Arrange
	configPath := writeTestConfig(t, "")

	// Act
	stdout, err := runOrderCommand(t, nil, configPath, "", "order", "get", "12345")

	// Assert
	require.Error(t, err)
	assert.Empty(t, stdout)
	var accountErr *apperr.AccountNotFoundError
	require.ErrorAs(t, err, &accountErr)
}

func TestOrderGetMissingOrderID(t *testing.T) {
	t.Parallel()

	// Arrange
	configPath := writeTestConfig(t, "hash123")

	// Act
	stdout, err := runOrderCommand(t, nil, configPath, "", "order", "get")

	// Assert
	require.Error(t, err)
	assert.Empty(t, stdout)
	var validationErr *apperr.ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, "order-id is required", validationErr.Error())
}

func TestOrderGetInvalidOrderID(t *testing.T) {
	t.Parallel()

	// Arrange
	configPath := writeTestConfig(t, "hash123")

	// Act
	stdout, err := runOrderCommand(t, nil, configPath, "", "order", "get", "abc")

	// Assert
	require.Error(t, err)
	assert.Empty(t, stdout)
	var validationErr *apperr.ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, "order-id must be a valid integer", validationErr.Error())
}

func TestOrderCancelSuccess(t *testing.T) {
	t.Parallel()

	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/trader/v1/accounts/hash123/orders/12345", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	configPath := writeTestConfigMutable(t, "hash123")
	cliClient := testClient(t, server)

	// Act
	stdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "cancel", "--confirm", "12345")

	// Assert
	require.NoError(t, err)
	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(12345), data["orderId"])
	assert.Equal(t, true, data["canceled"])
}

func TestOrderCancelMutableDisabled(t *testing.T) {
	t.Parallel()

	// Arrange
	configPath := writeTestConfig(t, "hash123")

	// Act
	stdout, err := runOrderCommand(t, nil, configPath, "", "order", "cancel", "--confirm", "12345")

	// Assert
	require.Error(t, err)
	assert.Empty(t, stdout)
	var validationErr *apperr.ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, mutableDisabledMessage, validationErr.Error())
}

func TestOrderCancelAPIError(t *testing.T) {
	t.Parallel()

	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/trader/v1/accounts/hash123/orders/12345", r.URL.Path)
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	configPath := writeTestConfigMutable(t, "hash123")
	cliClient := testClient(t, server)

	// Act
	stdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "cancel", "--confirm", "12345")

	// Assert
	require.Error(t, err)
	assert.Empty(t, stdout)
	var httpErr *apperr.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusInternalServerError, httpErr.StatusCode)
}

func TestOrderReplaceSuccess(t *testing.T) {
	t.Parallel()

	// Arrange
	var received models.OrderRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/trader/v1/accounts/hash123/orders/12345", r.URL.Path)

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &received))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	configPath := writeTestConfigMutable(t, "hash123")
	cliClient := testClient(t, server)

	// Act
	stdout, err := runOrderCommand(
		t,
		cliClient,
		configPath,
		"",
		"order", "replace", "12345",
		"--symbol", "AAPL",
		"--action", "BUY",
		"--quantity", "10",
		"--type", "LIMIT",
		"--price", "185.25",
		"--duration", "DAY",
		"--session", "NORMAL",
		"--confirm",
	)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, models.OrderTypeLimit, received.OrderType)
	require.NotNil(t, received.Price)
	assert.Equal(t, 185.25, *received.Price)
	require.Len(t, received.OrderLegCollection, 1)
	assert.Equal(t, models.InstructionBuy, received.OrderLegCollection[0].Instruction)
	assert.Equal(t, "AAPL", received.OrderLegCollection[0].Instrument.Symbol)

	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(12345), data["orderId"])
	assert.Equal(t, true, data["replaced"])
}

func TestOrderReplaceMutableDisabled(t *testing.T) {
	t.Parallel()

	// Arrange
	configPath := writeTestConfig(t, "hash123")

	// Act
	stdout, err := runOrderCommand(t, nil, configPath, "", "order", "replace", "12345", "--symbol", "AAPL", "--action", "BUY", "--quantity", "10", "--confirm")

	// Assert
	require.Error(t, err)
	assert.Empty(t, stdout)
	var validationErr *apperr.ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, mutableDisabledMessage, validationErr.Error())
}

func TestOrderReplaceAPIError(t *testing.T) {
	t.Parallel()

	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/trader/v1/accounts/hash123/orders/12345", r.URL.Path)
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	configPath := writeTestConfigMutable(t, "hash123")
	cliClient := testClient(t, server)

	// Act
	stdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "replace", "12345", "--symbol", "AAPL", "--action", "BUY", "--quantity", "10", "--confirm")

	// Assert
	require.Error(t, err)
	assert.Empty(t, stdout)
	var httpErr *apperr.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusInternalServerError, httpErr.StatusCode)
}

func TestParseRequiredOrderID(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		args      []string
		wantID    int64
		wantError string
	}{
		{name: "empty", args: []string{"order-test"}, wantError: "order-id is required"},
		{name: "valid int", args: []string{"order-test", "12345"}, wantID: 12345},
		{name: "invalid non-numeric", args: []string{"order-test", "abc"}, wantError: "order-id must be a valid integer"},
		{name: "negative int", args: []string{"order-test", "-99"}, wantError: "order-id must be a positive integer"},
		{name: "zero", args: []string{"order-test", "0"}, wantError: "order-id must be a positive integer"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var parsedID int64
			cmd := &cli.Command{
				Name: "order-test",
				Action: func(_ context.Context, cmd *cli.Command) error {
					var err error
					parsedID, err = parseRequiredOrderID(cmd)
					return err
				},
			}

			// Act
			err := runTestCommand(t, cmd, tc.args...)

			// Assert
			if tc.wantError == "" {
				require.NoError(t, err)
				assert.Equal(t, tc.wantID, parsedID)
				return
			}

			require.Error(t, err)
			var validationErr *apperr.ValidationError
			require.ErrorAs(t, err, &validationErr)
			assert.Equal(t, tc.wantError, validationErr.Error())
		})
	}
}

func TestOrderBuildOptionOutputsRequestJSON(t *testing.T) {
	t.Parallel()

	// Act
	stdout, err := runOrderCommand(
		t,
		nil,
		writeTestConfig(t, "hash123"),
		"",
		"order", "build", "option",
		"--underlying", "AAPL",
		"--expiration", testFutureExpDate,
		"--strike", "185",
		"--call",
		"--action", "BUY_TO_OPEN",
		"--quantity", "5",
		"--type", "LIMIT",
		"--price", "3.50",
		"--duration", "DAY",
		"--session", "NORMAL",
	)

	// Assert
	require.NoError(t, err)
	order := decodeOrderRequest(t, stdout)
	assert.Equal(t, models.OrderTypeLimit, order.OrderType)
	assert.Equal(t, models.OrderStrategyTypeSingle, order.OrderStrategyType)
	require.NotNil(t, order.Price)
	assert.Equal(t, 3.50, *order.Price)
	require.Len(t, order.OrderLegCollection, 1)
	leg := order.OrderLegCollection[0]
	assert.Equal(t, models.InstructionBuyToOpen, leg.Instruction)
	assert.Equal(t, 5.0, leg.Quantity)
	assert.Equal(t, models.AssetTypeOption, leg.Instrument.AssetType)
	wantOCC := orderbuilder.BuildOCCSymbol("AAPL", testFutureExpTime, 185, "CALL")
	assert.Equal(t, wantOCC, leg.Instrument.Symbol)
	require.NotNil(t, leg.Instrument.PutCall)
	assert.Equal(t, models.PutCallCall, *leg.Instrument.PutCall)
	require.NotNil(t, leg.Instrument.UnderlyingSymbol)
	assert.Equal(t, "AAPL", *leg.Instrument.UnderlyingSymbol)
	require.NotNil(t, leg.Instrument.OptionExpirationDate)
	assert.Equal(t, testFutureExpDate, *leg.Instrument.OptionExpirationDate)
	require.NotNil(t, leg.Instrument.OptionStrikePrice)
	assert.Equal(t, 185.0, *leg.Instrument.OptionStrikePrice)
}

func TestOrderBuildBracketOutputsRequestJSON(t *testing.T) {
	t.Parallel()

	// Act
	stdout, err := runOrderCommand(
		t,
		nil,
		writeTestConfig(t, "hash123"),
		"",
		"order", "build", "bracket",
		"--symbol", "AAPL",
		"--action", "BUY",
		"--quantity", "100",
		"--type", "LIMIT",
		"--price", "185.00",
		"--take-profit", "195.00",
		"--stop-loss", "175.00",
		"--duration", "DAY",
		"--session", "NORMAL",
	)

	// Assert
	require.NoError(t, err)
	order := decodeOrderRequest(t, stdout)
	assert.Equal(t, models.OrderStrategyTypeTrigger, order.OrderStrategyType)
	assert.Equal(t, models.OrderTypeLimit, order.OrderType)
	require.NotNil(t, order.Price)
	assert.Equal(t, 185.0, *order.Price)
	require.Len(t, order.OrderLegCollection, 1)
	assert.Equal(t, models.InstructionBuy, order.OrderLegCollection[0].Instruction)
	require.Len(t, order.ChildOrderStrategies, 1)
	assert.Equal(t, models.OrderStrategyTypeOCO, order.ChildOrderStrategies[0].OrderStrategyType)
	require.Len(t, order.ChildOrderStrategies[0].ChildOrderStrategies, 2)

	takeProfit := order.ChildOrderStrategies[0].ChildOrderStrategies[0]
	assert.Equal(t, models.OrderTypeLimit, takeProfit.OrderType)
	require.NotNil(t, takeProfit.Price)
	assert.Equal(t, 195.0, *takeProfit.Price)

	stopLoss := order.ChildOrderStrategies[0].ChildOrderStrategies[1]
	assert.Equal(t, models.OrderTypeStop, stopLoss.OrderType)
	require.NotNil(t, stopLoss.StopPrice)
	assert.Equal(t, 175.0, *stopLoss.StopPrice)
}
