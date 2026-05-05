package commands

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/client"
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

// runOrderCommand runs the Cobra order command tree with the provided args and stdin.
func runOrderCommand(t *testing.T, c *client.Ref, configPath, stdin string, args ...string) (string, error) {
	t.Helper()

	var stdout strings.Builder
	cmd := NewOrderCmd(c, configPath, &stdout)
	cmd.PersistentFlags().String("account", "", "Account hash value")
	cmd.PersistentFlags().String("config", configPath, "Path to config file")
	// Register --instruction/--order-type aliases (normally done by
	// RegisterOrderFlagAliasesOnTree in buildAppWithDeps after Setup).
	RegisterOrderFlagAliasesOnTree(cmd)
	cmd.SetIn(strings.NewReader(stdin))

	if len(args) > 0 && args[0] == "order" {
		args = args[1:]
	}

	cmd.SetArgs(args)
	err := cmd.Execute()

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

// writeTestOrderResponse writes a compact order fixture for action command tests.
func writeTestOrderResponse(t *testing.T, w http.ResponseWriter, orderID int64, status models.OrderStatus, symbol string) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	response := models.Order{
		OrderID:           &orderID,
		Status:            &status,
		OrderType:         models.OrderTypeLimit,
		OrderStrategyType: models.OrderStrategyTypeSingle,
		OrderLegCollection: []models.OrderLegCollection{{
			Instruction: models.InstructionBuy,
			Quantity:    10,
			Instrument: models.OrderInstrument{
				AssetType: models.AssetTypeEquity,
				Symbol:    symbol,
			},
		}},
	}
	require.NoError(t, json.NewEncoder(w).Encode(response))
}

func TestNewOrderCmdPreviewSpecModes(t *testing.T) {
	t.Parallel()

	orderID := int64(4242)
	previewResponse := models.PreviewOrder{OrderID: &orderID}
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

func TestNewOrderCmdSpecSemanticValidation(t *testing.T) {
	t.Parallel()

	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		assert.Failf(t, "unexpected API request for invalid spec", "%s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	}))
	defer server.Close()

	configPath := writeTestConfigMutable(t, "hash123")
	cliClient := &client.Ref{Client: client.NewClient("test-token", client.WithBaseURL(server.URL))}
	invalidLimitSpec := mustMarshalJSON(t, &models.OrderRequest{
		Session:           models.SessionNormal,
		Duration:          models.DurationDay,
		OrderType:         models.OrderTypeLimit,
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

	testCases := []struct {
		name string
		args []string
	}{
		{
			name: "preview",
			args: []string{"order", "preview", "--spec", invalidLimitSpec},
		},
		{
			name: "place",
			args: []string{"order", "place", "--spec", invalidLimitSpec},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := runOrderCommand(t, cliClient, configPath, "", testCase.args...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "LIMIT order requires a positive price")
		})
	}

	assert.Zero(t, requests)
}

func TestNewOrderCmdPreviewSpecAcceptsTriggerOrders(t *testing.T) {
	t.Parallel()

	var requestBodies []string
	orderID := int64(4244)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/trader/v1/accounts/hash123/previewOrder", r.URL.Path)

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		requestBodies = append(requestBodies, string(body))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(models.PreviewOrder{OrderID: &orderID}))
	}))
	defer server.Close()

	configPath := writeTestConfig(t, "hash123")
	cliClient := &client.Ref{Client: client.NewClient("test-token", client.WithBaseURL(server.URL))}
	bracket, err := orderbuilder.BuildBracketOrder(&orderbuilder.BracketParams{
		Symbol:     "AAPL",
		Action:     models.InstructionBuy,
		Quantity:   10,
		OrderType:  models.OrderTypeLimit,
		Price:      150,
		TakeProfit: 160,
		StopLoss:   140,
	})
	require.NoError(t, err)
	primary, err := orderbuilder.BuildEquityOrder(&orderbuilder.EquityParams{
		Symbol:    "MSFT",
		Action:    models.InstructionBuy,
		Quantity:  1,
		OrderType: models.OrderTypeMarket,
	})
	require.NoError(t, err)
	secondary, err := orderbuilder.BuildEquityOrder(&orderbuilder.EquityParams{
		Symbol:    "NVDA",
		Action:    models.InstructionBuy,
		Quantity:  1,
		OrderType: models.OrderTypeMarket,
	})
	require.NoError(t, err)
	fts, err := orderbuilder.BuildFTSOrder(&orderbuilder.FTSParams{Primary: *primary, Secondary: *secondary})
	require.NoError(t, err)

	testCases := []struct {
		name string
		spec string
	}{
		{name: "bracket", spec: mustMarshalJSON(t, bracket)},
		{name: "fts", spec: mustMarshalJSON(t, &fts)},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			stdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "preview", "--spec", testCase.spec)
			require.NoError(t, err)

			envelope := decodeEnvelope(t, stdout)
			data, ok := envelope.Data.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, float64(4244), data["orderId"])
		})
	}

	require.Len(t, requestBodies, len(testCases))
	for _, body := range requestBodies {
		var order models.OrderRequest
		require.NoError(t, json.Unmarshal([]byte(body), &order))
		assert.Equal(t, models.OrderStrategyTypeTrigger, order.OrderStrategyType)
		require.NotEmpty(t, order.OrderLegCollection)
		require.NotEmpty(t, order.ChildOrderStrategies)
	}
}

func TestNewOrderCmdPreviewTypedSubcommands(t *testing.T) {
	orderID := int64(4243)
	previewResponse := models.PreviewOrder{OrderID: &orderID}
	type requestRecord struct {
		Path string
		Body string
	}
	requestBodies := make([]requestRecord, 0, 4)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/trader/v1/accounts/hash123/previewOrder", r.URL.Path)

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		requestBodies = append(requestBodies, requestRecord{Path: r.URL.Path, Body: string(body)})

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(previewResponse))
	}))
	defer server.Close()

	configPath := writeTestConfig(t, "hash123")
	cliClient := &client.Ref{Client: client.NewClient("test-token", client.WithBaseURL(server.URL))}

	testCases := []struct {
		name       string
		args       []string
		assertBody func(*testing.T, models.OrderRequest)
	}{
		{
			name: "equity",
			args: []string{"order", "preview", "equity", "--symbol", "AAPL", "--action", "BUY", "--quantity", "10", "--type", "LIMIT", "--price", "185.25"},
			assertBody: func(t *testing.T, order models.OrderRequest) {
				t.Helper()

				assert.Equal(t, models.OrderTypeLimit, order.OrderType)
				require.Len(t, order.OrderLegCollection, 1)
				assert.Equal(t, "AAPL", order.OrderLegCollection[0].Instrument.Symbol)
				assert.Equal(t, models.InstructionBuy, order.OrderLegCollection[0].Instruction)
			},
		},
		{
			name: "option",
			args: []string{"order", "preview", "option", "--underlying", "AAPL", "--expiration", testFutureExpDate, "--strike", "200", "--call", "--action", "BUY_TO_OPEN", "--quantity", "1", "--type", "LIMIT", "--price", "5.00"},
			assertBody: func(t *testing.T, order models.OrderRequest) {
				t.Helper()

				assert.Equal(t, models.OrderTypeLimit, order.OrderType)
				require.Len(t, order.OrderLegCollection, 1)
				assert.Equal(t, models.AssetTypeOption, order.OrderLegCollection[0].Instrument.AssetType)
				assert.Contains(t, order.OrderLegCollection[0].Instrument.Symbol, "AAPL")
				assert.Equal(t, models.InstructionBuyToOpen, order.OrderLegCollection[0].Instruction)
			},
		},
		{
			name: "bracket",
			args: []string{"order", "preview", "bracket", "--symbol", "NVDA", "--action", "BUY", "--quantity", "10", "--type", "MARKET", "--take-profit", "150", "--stop-loss", "120"},
			assertBody: func(t *testing.T, order models.OrderRequest) {
				t.Helper()

				assert.Equal(t, models.OrderStrategyTypeTrigger, order.OrderStrategyType)
				require.Len(t, order.OrderLegCollection, 1)
				assert.Equal(t, "NVDA", order.OrderLegCollection[0].Instrument.Symbol)
				require.NotEmpty(t, order.ChildOrderStrategies)
			},
		},
		{
			name: "oco",
			args: []string{"order", "preview", "oco", "--symbol", "TSLA", "--action", "SELL", "--quantity", "5", "--take-profit", "250", "--stop-loss", "200"},
			assertBody: func(t *testing.T, order models.OrderRequest) {
				t.Helper()

				assert.Equal(t, models.OrderStrategyTypeOCO, order.OrderStrategyType)
				require.Len(t, order.ChildOrderStrategies, 2)
			},
		},
	}

	for index, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			stdout, err := runOrderCommand(t, cliClient, configPath, "", testCase.args...)
			require.NoError(t, err)

			envelope := decodeEnvelope(t, stdout)
			data, ok := envelope.Data.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, float64(4243), data["orderId"])
			require.Contains(t, data, "builtOrder")
			require.Contains(t, data, "preview")

			var order models.OrderRequest
			require.NoError(t, json.Unmarshal([]byte(requestBodies[index].Body), &order))
			testCase.assertBody(t, order)
		})
	}

	require.Len(t, requestBodies, len(testCases))
}

func TestNewOrderCmdBuildEquityOutputsRequestJSON(t *testing.T) {
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

func TestNewOrderCmdBuildNewOptionStrategiesOutputRequestJSON(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		args        []string
		complexType models.ComplexOrderStrategyType
		orderType   models.OrderType
		legs        int
	}{
		{
			name:        "butterfly",
			args:        []string{"order", "build", "butterfly", "--underlying", "F", "--expiration", testFutureExpDate, "--lower-strike", "10", "--middle-strike", "12", "--upper-strike", "14", "--call", "--buy", "--open", "--quantity", "1", "--price", "0.50"},
			complexType: models.ComplexOrderStrategyTypeButterfly, orderType: models.OrderTypeNetDebit, legs: 3,
		},
		{
			name:        "condor",
			args:        []string{"order", "build", "condor", "--underlying", "F", "--expiration", testFutureExpDate, "--lower-strike", "10", "--lower-middle-strike", "12", "--upper-middle-strike", "14", "--upper-strike", "16", "--put", "--sell", "--open", "--quantity", "1", "--price", "0.75"},
			complexType: models.ComplexOrderStrategyTypeCondor, orderType: models.OrderTypeNetCredit, legs: 4,
		},
		{
			name:        "vertical-roll",
			args:        []string{"order", "build", "vertical-roll", "--underlying", "F", "--close-expiration", testFutureExpDate, "--open-expiration", testFutureExpTime.AddDate(0, 1, 0).Format("2006-01-02"), "--close-long-strike", "12", "--close-short-strike", "14", "--open-long-strike", "13", "--open-short-strike", "15", "--call", "--credit", "--quantity", "1", "--price", "0.25"},
			complexType: models.ComplexOrderStrategyTypeVerticalRoll, orderType: models.OrderTypeNetCredit, legs: 4,
		},
		{
			name:        "back-ratio",
			args:        []string{"order", "build", "back-ratio", "--underlying", "F", "--expiration", testFutureExpDate, "--short-strike", "12", "--long-strike", "14", "--call", "--open", "--quantity", "1", "--debit", "--price", "0.20"},
			complexType: models.ComplexOrderStrategyTypeBackRatio, orderType: models.OrderTypeNetDebit, legs: 2,
		},
		{
			name:        "double-diagonal",
			args:        []string{"order", "build", "double-diagonal", "--underlying", "F", "--near-expiration", testFutureExpDate, "--far-expiration", testFutureExpTime.AddDate(0, 1, 0).Format("2006-01-02"), "--put-far-strike", "9", "--put-near-strike", "10", "--call-near-strike", "14", "--call-far-strike", "15", "--open", "--quantity", "1", "--price", "0.80"},
			complexType: models.ComplexOrderStrategyTypeDoubleDiagonal, orderType: models.OrderTypeNetDebit, legs: 4,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			stdout, err := runOrderCommand(t, nil, writeTestConfig(t, "hash123"), "", testCase.args...)
			require.NoError(t, err)

			order := decodeOrderRequest(t, stdout)
			assert.Equal(t, models.OrderStrategyTypeSingle, order.OrderStrategyType)
			assert.Equal(t, testCase.orderType, order.OrderType)
			require.NotNil(t, order.ComplexOrderStrategyType)
			assert.Equal(t, testCase.complexType, *order.ComplexOrderStrategyType)
			require.Len(t, order.OrderLegCollection, testCase.legs)
		})
	}
}

func TestNewOrderCmdPlaceEquityPipeline(t *testing.T) {
	t.Parallel()

	var received models.OrderRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/trader/v1/accounts/default-hash/orders":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &received))

			w.Header().Set("Location", "/trader/v1/accounts/default-hash/orders/67890")
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && r.URL.Path == "/trader/v1/accounts/default-hash/orders/67890":
			writeTestOrderResponse(t, w, 67890, models.OrderStatusQueued, "AAPL")
		default:
			assert.Failf(t, "unexpected request", "%s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
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
	assert.Equal(t, "place", data["action"])
	assert.Equal(t, float64(67890), data["orderId"])
	assert.Equal(t, "verified", data["verificationState"])
	assert.Equal(t, "QUEUED", data["orderStatus"])
	submittedOrder, ok := data["submittedOrder"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "LIMIT", submittedOrder["orderType"])
	order, ok := data["order"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(67890), order["orderId"])
	assert.Equal(t, "QUEUED", order["status"])
	assert.NotEmpty(t, envelope.Metadata.Timestamp)
}

func TestNewOrderCmdPlaceSpecFromFile(t *testing.T) {
	t.Parallel()

	var received models.OrderRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/trader/v1/accounts/hash123/orders":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &received))

			w.Header().Set("Location", "/trader/v1/accounts/hash123/orders/24680")
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && r.URL.Path == "/trader/v1/accounts/hash123/orders/24680":
			writeTestOrderResponse(t, w, 24680, models.OrderStatusQueued, "MSFT")
		default:
			assert.Failf(t, "unexpected request", "%s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
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

	stdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "place", "--spec", "@"+specPath)
	require.NoError(t, err)

	require.Len(t, received.OrderLegCollection, 1)
	assert.Equal(t, "MSFT", received.OrderLegCollection[0].Instrument.Symbol)
	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "place", data["action"])
	assert.Equal(t, float64(24680), data["orderId"])
	assert.Equal(t, "verified", data["verificationState"])
	assert.Equal(t, "QUEUED", data["orderStatus"])
	submittedOrder, ok := data["submittedOrder"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "MARKET", submittedOrder["orderType"])
	order, ok := data["order"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(24680), order["orderId"])
	assert.Equal(t, "QUEUED", order["status"])
}

func TestNewOrderCmdPreviewSaveAndPlaceFromPreview(t *testing.T) {
	orderID := int64(13579)
	previewResponse := models.PreviewOrder{OrderID: &orderID}
	var previewedOrder models.OrderRequest
	var placedOrder models.OrderRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/trader/v1/accounts/hash123/previewOrder":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &previewedOrder))

			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(previewResponse))
		case r.Method == http.MethodPost && r.URL.Path == "/trader/v1/accounts/hash123/orders":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &placedOrder))

			w.Header().Set("Location", "/trader/v1/accounts/hash123/orders/13579")
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && r.URL.Path == "/trader/v1/accounts/hash123/orders/13579":
			writeTestOrderResponse(t, w, 13579, models.OrderStatusQueued, "AAPL")
		default:
			assert.Failf(t, "unexpected request", "%s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	t.Setenv("SCHWAB_AGENT_STATE_DIR", t.TempDir())
	configPath := writeTestConfigMutable(t, "hash123")
	cliClient := &client.Ref{Client: client.NewClient("test-token", client.WithBaseURL(server.URL))}

	previewStdout, err := runOrderCommand(
		t,
		cliClient,
		configPath,
		"",
		"order", "preview", "equity",
		"--symbol", "AAPL",
		"--action", "BUY",
		"--quantity", "10",
		"--type", "LIMIT",
		"--price", "185.25",
		"--save-preview",
	)
	require.NoError(t, err)

	previewEnvelope := decodeEnvelope(t, previewStdout)
	previewData, ok := previewEnvelope.Data.(map[string]any)
	require.True(t, ok)
	digestData, ok := previewData["previewDigest"].(map[string]any)
	require.True(t, ok)
	digest, ok := digestData["digest"].(string)
	require.True(t, ok)
	require.Len(t, digest, 64)
	assert.Equal(t, "hash123", digestData["account"])
	assert.Equal(t, "order.place", digestData["operation"])

	placeStdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "place", "--from-preview", digest)
	require.NoError(t, err)

	assert.Equal(t, previewedOrder, placedOrder)
	placeEnvelope := decodeEnvelope(t, placeStdout)
	placeData, ok := placeEnvelope.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "place", placeData["action"])
	assert.Equal(t, float64(13579), placeData["orderId"])
	assert.Equal(t, digest, placeData["previewDigest"])
	assert.Equal(t, "verified", placeData["verificationState"])
}

func TestNewOrderCmdPlaceFromPreviewRejectsAccountMismatch(t *testing.T) {
	t.Setenv("SCHWAB_AGENT_STATE_DIR", t.TempDir())
	configPath := writeTestConfigMutable(t, "hash123")
	cliClient := &client.Ref{}
	digestData, err := saveOrderPreview("hash123", testPreviewLedgerOrder(t), nil)
	require.NoError(t, err)

	_, err = runOrderCommand(
		t,
		cliClient,
		configPath,
		"",
		"order", "place",
		"--account", "hash999",
		"--from-preview", digestData.Digest,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}

func TestNewOrderCmdPlaceNoLocationReturnsPartialSuccess(t *testing.T) {
	t.Parallel()

	var getRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/trader/v1/accounts/hash123/orders":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet:
			getRequests++
			assert.Failf(t, "unexpected follow-up GET", "request without order ID: %s", r.URL.Path)
			http.NotFound(w, r)
		default:
			assert.Failf(t, "unexpected request", "%s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
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

	stdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "place", "--spec", mustMarshalJSON(t, orderRequest))
	require.NoError(t, err)

	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "place", data["action"])
	assert.Equal(t, float64(0), data["orderId"])
	assert.Equal(t, "unverified", data["verificationState"])
	verificationFailures, ok := data["verificationFailures"].([]any)
	require.True(t, ok)
	require.Len(t, verificationFailures, 1)
	assert.Contains(t, verificationFailures[0], "did not return an order ID")
	assert.NotContains(t, data, "order")
	assert.Equal(t, 0, getRequests)
	require.Len(t, envelope.Errors, 1)
	assert.Contains(t, envelope.Errors[0], "did not return an order ID")
}

func TestNewOrderCmdPlaceUnknownFlagSuggestsSubcommand(t *testing.T) {
	t.Parallel()

	// Arrange
	configPath := writeTestConfigMutable(t, "test-account-hash")

	// Act
	stdout, err := runOrderCommand(t, nil, configPath, "", "order", "place", "--symbol", "AAPL")

	// Assert
	require.Error(t, err)
	assert.Empty(t, stdout)

	var valErr *apperr.ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Contains(t, valErr.Error(), "order place")
	assert.Contains(t, valErr.Error(), "requires a subcommand")
	assert.Contains(t, valErr.Error(), "equity")
	assert.Contains(t, valErr.Error(), "option")
	assert.Contains(t, valErr.Error(), "bracket")
	assert.Contains(t, valErr.Error(), "oco")
	assert.Contains(t, valErr.Error(), "unknown flag: --symbol")
}

func TestNewOrderCmdPlaceSpecMissingValueKeepsOriginalUsageError(t *testing.T) {
	t.Parallel()

	// Arrange
	configPath := writeTestConfigMutable(t, "test-account-hash")

	// Act
	stdout, err := runOrderCommand(t, nil, configPath, "", "order", "place", "--spec")

	// Assert
	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.NotContains(t, err.Error(), "requires a subcommand")
}

func TestNewOrderCmdMutableGuard(t *testing.T) {
	t.Parallel()

	// Config WITHOUT i-also-like-to-live-dangerously (default: false).
	configPath := writeTestConfig(t, "hash123")

	testCases := []struct {
		name string
		args []string
	}{
		{
			name: "place spec blocked without mutable flag",
			args: []string{"order", "place", "--spec", `{"session":"NORMAL","duration":"DAY","orderType":"MARKET","orderStrategyType":"SINGLE","orderLegCollection":[{"instruction":"BUY","quantity":1,"instrument":{"assetType":"EQUITY","symbol":"AAPL"}}]}`},
		},
		{
			name: "place equity blocked without mutable flag",
			args: []string{"order", "place", "equity", "--symbol", "AAPL", "--action", "BUY", "--quantity", "10"},
		},
		{
			name: "cancel blocked without mutable flag",
			args: []string{"order", "cancel", "12345"},
		},
		{
			name: "replace blocked without mutable flag",
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
			assert.Contains(t, validationErr.Error(), "i-also-like-to-live-dangerously")
		})
	}
}

func TestNewOrderCmdMutableGuardRequired(t *testing.T) {
	t.Parallel()

	// Config WITHOUT mutable flag.
	// The mutable guard should block the operation.
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

func TestNewOrderCmdPreviewNotBlockedByMutableGuard(t *testing.T) {
	t.Parallel()

	orderID := int64(9999)
	previewResponse := models.PreviewOrder{OrderID: &orderID}
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

func TestNewOrderCmdBuildOCOOutputsRequestJSON(t *testing.T) {
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

func TestNewOrderCmdBuildOCOSingleExit(t *testing.T) {
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

func TestNewOrderCmdPlaceOCOPipeline(t *testing.T) {
	t.Parallel()

	var received models.OrderRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/trader/v1/accounts/default-hash/orders":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &received))

			w.Header().Set("Location", "/trader/v1/accounts/default-hash/orders/55555")
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && r.URL.Path == "/trader/v1/accounts/default-hash/orders/55555":
			writeTestOrderResponse(t, w, 55555, models.OrderStatusQueued, "AAPL")
		default:
			assert.Failf(t, "unexpected request", "%s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
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
	)
	require.NoError(t, err)

	// Verify the OCO structure was sent to the API.
	assert.Equal(t, models.OrderStrategyTypeOCO, received.OrderStrategyType)
	require.Len(t, received.ChildOrderStrategies, 2)

	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(55555), data["orderId"])
	order, ok := data["order"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(55555), order["orderId"])
}

// --- Spread build tests (iron condor, vertical, strangle, straddle, covered call) ---
//
// These tests exercise the full CLI pipeline: CLI flags → parseXxxParams → validate → build → JSON.
// The builder functions are independently tested in orderbuilder/spread_test.go; these tests verify
// the parsing layer wires flags to params correctly, which is the untested gap that matters most
// for order correctness.

func TestNewOrderCmdBuildIronCondorOpen(t *testing.T) {
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

func TestNewOrderCmdBuildIronCondorClose(t *testing.T) {
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

func TestNewOrderCmdBuildVerticalCallDebit(t *testing.T) {
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

func TestNewOrderCmdBuildVerticalPutCredit(t *testing.T) {
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

func TestNewOrderCmdBuildStrangleBuyOpen(t *testing.T) {
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

func TestNewOrderCmdBuildStrangleSellOpen(t *testing.T) {
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

func TestNewOrderCmdBuildStraddleBuyOpen(t *testing.T) {
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

func TestNewOrderCmdBuildStraddleSellOpen(t *testing.T) {
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

func TestNewOrderCmdBuildCoveredCallOutputsRequestJSON(t *testing.T) {
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

func TestNewOrderCmdBuildSpreadDefaultDurationSession(t *testing.T) {
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
func TestNewOrderCmdBuildParseErrors(t *testing.T) {
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
			wantMsg: "invalid action",
		},
		{
			name: "option invalid action",
			args: []string{
				"order", "build", "option",
				"--underlying", "AAPL", "--action", "YOLO",
				"--call", "--expiration", "2026-06-19",
				"--strike", "150", "--quantity", "1",
			},
			wantMsg: "invalid action",
		},
		{
			name: "bracket invalid action",
			args: []string{
				"order", "build", "bracket",
				"--symbol", "AAPL", "--action", "YOLO",
				"--quantity", "10", "--take-profit", "200",
			},
			wantMsg: "invalid action",
		},
		{
			name: "oco invalid action",
			args: []string{
				"order", "build", "oco",
				"--symbol", "AAPL", "--action", "YOLO",
				"--quantity", "10", "--stop-loss", "140",
			},
			wantMsg: "invalid action",
		},
		// parseOrderType invalid value (exercises default branch).
		{
			name: "equity invalid order type",
			args: []string{
				"order", "build", "equity",
				"--symbol", "AAPL", "--action", "BUY",
				"--quantity", "10", "--type", "BANANAS",
			},
			wantMsg: "invalid type",
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
			wantMsg: "invalid duration",
		},
		{
			name: "oco invalid session",
			args: []string{
				"order", "build", "oco",
				"--symbol", "AAPL", "--action", "SELL",
				"--quantity", "10", "--stop-loss", "140",
				"--session", "MIDNIGHT",
			},
			wantMsg: "invalid session",
		},
		// parseDurationSession error in equity path.
		{
			name: "equity invalid duration",
			args: []string{
				"order", "build", "equity",
				"--symbol", "AAPL", "--action", "BUY",
				"--quantity", "10", "--duration", "FOREVER",
			},
			wantMsg: "invalid duration",
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
			wantMsg: "invalid duration",
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
			wantMsg: "at least one of the flags in the group [call put] is required",
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
			wantMsg: "invalid session",
		},
		// parseStopPriceLinkBasis invalid value.
		{
			name: "equity invalid stop-link-basis",
			args: []string{
				"order", "build", "equity",
				"--symbol", "AAPL", "--action", "SELL",
				"--quantity", "10", "--type", "TRAILING_STOP",
				"--stop-offset", "2.50", "--stop-link-basis", "NOPE",
			},
			wantMsg: "invalid stop-link-basis",
		},
		// parseStopPriceLinkType invalid value.
		{
			name: "equity invalid stop-link-type",
			args: []string{
				"order", "build", "equity",
				"--symbol", "AAPL", "--action", "SELL",
				"--quantity", "10", "--type", "TRAILING_STOP",
				"--stop-offset", "2.50", "--stop-link-type", "NOPE",
			},
			wantMsg: "invalid stop-link-type",
		},
		// parseStopType invalid value.
		{
			name: "equity invalid stop-type",
			args: []string{
				"order", "build", "equity",
				"--symbol", "AAPL", "--action", "SELL",
				"--quantity", "10", "--type", "TRAILING_STOP",
				"--stop-offset", "2.50", "--stop-type", "NOPE",
			},
			wantMsg: "invalid stop-type",
		},
		// parseSpecialInstruction invalid value.
		{
			name: "equity invalid special-instruction",
			args: []string{
				"order", "build", "equity",
				"--symbol", "AAPL", "--action", "BUY",
				"--quantity", "10", "--special-instruction", "NOPE",
			},
			wantMsg: "invalid special-instruction",
		},
		{
			name: "option invalid special-instruction",
			args: []string{
				"order", "build", "option",
				"--underlying", "AAPL", "--action", "BUY_TO_OPEN",
				"--call", "--expiration", "2026-06-19",
				"--strike", "150", "--quantity", "1",
				"--special-instruction", "NOPE",
			},
			wantMsg: "invalid special-instruction",
		},
		// parseDestination invalid value.
		{
			name: "equity invalid destination",
			args: []string{
				"order", "build", "equity",
				"--symbol", "AAPL", "--action", "BUY",
				"--quantity", "10", "--type", "LIMIT",
				"--price", "150", "--destination", "NOPE",
			},
			wantMsg: "invalid destination",
		},
		{
			name: "option invalid destination",
			args: []string{
				"order", "build", "option",
				"--underlying", "AAPL", "--action", "BUY_TO_OPEN",
				"--call", "--expiration", "2026-06-19",
				"--strike", "150", "--quantity", "1",
				"--destination", "NOPE",
			},
			wantMsg: "invalid destination",
		},
		// parsePriceLinkBasis invalid value.
		{
			name: "equity invalid price-link-basis",
			args: []string{
				"order", "build", "equity",
				"--symbol", "AAPL", "--action", "BUY",
				"--quantity", "10", "--type", "LIMIT",
				"--price", "150", "--price-link-basis", "NOPE",
			},
			wantMsg: "invalid price-link-basis",
		},
		{
			name: "option invalid price-link-basis",
			args: []string{
				"order", "build", "option",
				"--underlying", "AAPL", "--action", "BUY_TO_OPEN",
				"--call", "--expiration", "2026-06-19",
				"--strike", "150", "--quantity", "1",
				"--price-link-basis", "NOPE",
			},
			wantMsg: "invalid price-link-basis",
		},
		// parsePriceLinkType invalid value.
		{
			name: "equity invalid price-link-type",
			args: []string{
				"order", "build", "equity",
				"--symbol", "AAPL", "--action", "BUY",
				"--quantity", "10", "--type", "LIMIT",
				"--price", "150", "--price-link-type", "NOPE",
			},
			wantMsg: "invalid price-link-type",
		},
		{
			name: "option invalid price-link-type",
			args: []string{
				"order", "build", "option",
				"--underlying", "AAPL", "--action", "BUY_TO_OPEN",
				"--call", "--expiration", "2026-06-19",
				"--strike", "150", "--quantity", "1",
				"--price-link-type", "NOPE",
			},
			wantMsg: "invalid price-link-type",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stdout, err := runOrderCommand(t, nil, writeTestConfig(t, "hash123"), "", tc.args...)
			require.Error(t, err)
			assert.Empty(t, stdout)

			assert.ErrorContains(t, err, tc.wantMsg)
		})
	}
}

func TestNewOrderCmdBuildSpreadParseErrors(t *testing.T) {
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
			wantMsg: "at least one of the flags in the group [open close] is required",
		},
		{
			name: "vertical missing open/close",
			args: []string{
				"order", "build", "vertical",
				"--underlying", "AAPL", "--expiration", testFutureExpDate,
				"--long-strike", "180", "--short-strike", "190",
				"--call", "--quantity", "1", "--price", "2.00",
			},
			wantMsg: "at least one of the flags in the group [open close] is required",
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
			wantMsg: "at least one of the flags in the group [buy sell] is required",
		},
		{
			name: "straddle missing buy/sell",
			args: []string{
				"order", "build", "straddle",
				"--underlying", "NVDA", "--expiration", testFutureExpDate,
				"--strike", "130",
				"--open", "--quantity", "1", "--price", "15.00",
			},
			wantMsg: "at least one of the flags in the group [buy sell] is required",
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
			wantMsg: "at least one of the flags in the group [call put] is required",
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
			wantMsg: "invalid duration",
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
			wantMsg: "invalid session",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stdout, err := runOrderCommand(t, nil, writeTestConfig(t, "hash123"), "", tc.args...)
			require.Error(t, err)
			assert.Empty(t, stdout)

			assert.ErrorContains(t, err, tc.wantMsg)
		})
	}
}

func TestNewOrderCmdListAllAccounts(t *testing.T) {
	t.Parallel()

	// Arrange - mix of terminal and non-terminal orders. Default listing
	// should filter out terminal statuses (FILLED, CANCELED, etc.).
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/trader/v1/orders", r.URL.Path)
		assert.NotEmpty(t, r.URL.Query().Get("fromEnteredTime"))
		assert.NotEmpty(t, r.URL.Query().Get("toEnteredTime"))

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`[
			{"session":"NORMAL","duration":"DAY","orderType":"LIMIT","orderStrategyType":"SINGLE","orderId":111,"status":"WORKING","orderLegCollection":[{"instruction":"BUY","quantity":10,"instrument":{"assetType":"EQUITY","symbol":"AAPL"}}]},
			{"session":"NORMAL","duration":"DAY","orderType":"MARKET","orderStrategyType":"SINGLE","orderId":222,"status":"FILLED","orderLegCollection":[{"instruction":"SELL","quantity":5,"instrument":{"assetType":"EQUITY","symbol":"MSFT"}}]},
			{"session":"NORMAL","duration":"DAY","orderType":"LIMIT","orderStrategyType":"SINGLE","orderId":333,"status":"QUEUED","orderLegCollection":[{"instruction":"BUY","quantity":20,"instrument":{"assetType":"EQUITY","symbol":"GOOG"}}]}
		]`))
		require.NoError(t, err)
	}))
	defer server.Close()

	configPath := writeTestConfig(t, "hash123")
	cliClient := testClient(t, server)

	// Act
	stdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "list")

	// Assert - FILLED order should be filtered out by default
	require.NoError(t, err)
	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	orders, ok := data["orders"].([]any)
	require.True(t, ok)
	require.Len(t, orders, 2, "should exclude terminal FILLED order")

	first, ok := orders[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(111), first["orderId"])
	assert.Equal(t, "WORKING", first["status"])

	second, ok := orders[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(333), second["orderId"])
	assert.Equal(t, "QUEUED", second["status"])
}

func TestNewOrderCmdListWithAccount(t *testing.T) {
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

func TestNewOrderCmdListWithFilters(t *testing.T) {
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

func TestNewOrderCmdListMultipleStatuses(t *testing.T) {
	t.Parallel()

	// Arrange - the Schwab API accepts only one status per request, so multiple
	// --status flags should produce separate API calls with merged results.
	var requestStatuses []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/trader/v1/orders", r.URL.Path)
		status := r.URL.Query().Get("status")
		requestStatuses = append(requestStatuses, status)

		w.Header().Set("Content-Type", "application/json")
		switch status {
		case "WORKING":
			_, err := w.Write([]byte(`[{"session":"NORMAL","duration":"DAY","orderType":"LIMIT","orderStrategyType":"SINGLE","orderId":111,"status":"WORKING","orderLegCollection":[{"instruction":"BUY","quantity":10,"instrument":{"assetType":"EQUITY","symbol":"AAPL"}}]}]`))
			require.NoError(t, err)
		case "FILLED":
			_, err := w.Write([]byte(`[{"session":"NORMAL","duration":"DAY","orderType":"MARKET","orderStrategyType":"SINGLE","orderId":222,"status":"FILLED","orderLegCollection":[{"instruction":"SELL","quantity":5,"instrument":{"assetType":"EQUITY","symbol":"MSFT"}}]}]`))
			require.NoError(t, err)
		default:
			assert.Failf(t, "unexpected status filter", "got status %q", status)
		}
	}))
	defer server.Close()

	configPath := writeTestConfig(t, "hash123")
	cliClient := testClient(t, server)

	// Act
	stdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "list", "--status", "WORKING", "--status", "FILLED")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"WORKING", "FILLED"}, requestStatuses, "should fan out one request per status")
	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	orders, ok := data["orders"].([]any)
	require.True(t, ok)
	require.Len(t, orders, 2)

	first, ok := orders[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(111), first["orderId"])
	assert.Equal(t, "WORKING", first["status"])

	second, ok := orders[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(222), second["orderId"])
	assert.Equal(t, "FILLED", second["status"])
}

func TestNewOrderCmdListCommaSeparatedStatuses(t *testing.T) {
	t.Parallel()

	// Arrange - comma-separated statuses should split into separate API calls,
	// matching the --fields pattern in quote.go.
	var requestStatuses []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := r.URL.Query().Get("status")
		requestStatuses = append(requestStatuses, status)

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`[]`))
		require.NoError(t, err)
	}))
	defer server.Close()

	configPath := writeTestConfig(t, "hash123")
	cliClient := testClient(t, server)

	// Act
	stdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "list", "--status", "WORKING,FILLED")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"WORKING", "FILLED"}, requestStatuses, "comma-separated values should produce separate API calls")
	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	orders, ok := data["orders"].([]any)
	require.True(t, ok)
	assert.Empty(t, orders)
}

func TestNewOrderCmdListAPIError(t *testing.T) {
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

func TestNewOrderCmdListDefaultFiltersTerminalStatuses(t *testing.T) {
	t.Parallel()

	// Arrange - return orders in every terminal status plus one non-terminal.
	// Default listing (no --status flag) should filter out all terminal ones.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`[
			{"orderId":1,"status":"WORKING"},
			{"orderId":2,"status":"FILLED"},
			{"orderId":3,"status":"CANCELED"},
			{"orderId":4,"status":"REJECTED"},
			{"orderId":5,"status":"EXPIRED"},
			{"orderId":6,"status":"REPLACED"},
			{"orderId":7,"status":"QUEUED"},
			{"orderId":8,"status":"PENDING_ACTIVATION"}
		]`))
		require.NoError(t, err)
	}))
	defer server.Close()

	configPath := writeTestConfig(t, "hash123")
	cliClient := testClient(t, server)

	// Act
	stdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "list")

	// Assert - only non-terminal orders should remain
	require.NoError(t, err)
	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	orders, ok := data["orders"].([]any)
	require.True(t, ok)
	require.Len(t, orders, 3, "should keep WORKING, QUEUED, PENDING_ACTIVATION")

	expectedIDs := []float64{1, 7, 8}
	for i, order := range orders {
		o, ok := order.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, expectedIDs[i], o["orderId"])
	}
}

func TestNewOrderCmdListStatusAllDisablesFiltering(t *testing.T) {
	t.Parallel()

	// Arrange - --status all should return everything unfiltered.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// "all" is a pseudo-status handled client-side, not sent to the API.
		assert.Empty(t, r.URL.Query().Get("status"), "should not send 'all' as API status")

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`[
			{"orderId":1,"status":"WORKING"},
			{"orderId":2,"status":"FILLED"},
			{"orderId":3,"status":"CANCELED"}
		]`))
		require.NoError(t, err)
	}))
	defer server.Close()

	configPath := writeTestConfig(t, "hash123")
	cliClient := testClient(t, server)

	// Act
	stdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "list", "--status", "all")

	// Assert - all orders should appear, no filtering
	require.NoError(t, err)
	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	orders, ok := data["orders"].([]any)
	require.True(t, ok)
	require.Len(t, orders, 3, "should return all orders when --status all")
}

func TestNewOrderCmdListRecentIncludesTerminalStatuses(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/trader/v1/orders", r.URL.Path)
		assert.Empty(t, r.URL.Query().Get("status"), "recent with no explicit status should request all activity")

		from := r.URL.Query().Get("fromEnteredTime")
		require.NotEmpty(t, from)
		parsedFrom, err := time.Parse(time.RFC3339, from)
		require.NoError(t, err)
		assert.WithinDuration(t, time.Now().UTC().Add(-24*time.Hour), parsedFrom, 2*time.Second)

		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write([]byte(`[
			{"orderId":1,"status":"FILLED"},
			{"orderId":2,"status":"CANCELED"},
			{"orderId":3,"status":"WORKING"}
		]`))
		require.NoError(t, err)
	}))
	defer server.Close()

	configPath := writeTestConfig(t, "hash123")
	cliClient := testClient(t, server)

	stdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "list", "--recent")
	require.NoError(t, err)

	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	orders, ok := data["orders"].([]any)
	require.True(t, ok)
	require.Len(t, orders, 3, "recent mode should retain terminal activity")
}

func TestNewOrderCmdListExplicitStatusBypassesDefault(t *testing.T) {
	t.Parallel()

	// Arrange - when the user explicitly requests a terminal status,
	// it should be passed to the API and returned without client-side filtering.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "FILLED", r.URL.Query().Get("status"))

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`[{"orderId":1,"status":"FILLED"},{"orderId":2,"status":"FILLED"}]`))
		require.NoError(t, err)
	}))
	defer server.Close()

	configPath := writeTestConfig(t, "hash123")
	cliClient := testClient(t, server)

	// Act
	stdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "list", "--status", "FILLED")

	// Assert - explicit status request returns all matching, no default filter
	require.NoError(t, err)
	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	orders, ok := data["orders"].([]any)
	require.True(t, ok)
	require.Len(t, orders, 2)
}

func TestNewOrderCmdListNilStatusIncludedByDefault(t *testing.T) {
	t.Parallel()

	// Arrange - orders with no status field should be kept (conservative:
	// don't hide orders with unknown state).
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`[
			{"orderId":1},
			{"orderId":2,"status":"FILLED"}
		]`))
		require.NoError(t, err)
	}))
	defer server.Close()

	configPath := writeTestConfig(t, "hash123")
	cliClient := testClient(t, server)

	// Act
	stdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "list")

	// Assert - nil-status order kept, FILLED filtered
	require.NoError(t, err)
	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	orders, ok := data["orders"].([]any)
	require.True(t, ok)
	require.Len(t, orders, 1)
	o, ok := orders[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(1), o["orderId"])
}

func TestNewOrderCmdGetSuccess(t *testing.T) {
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

func TestNewOrderCmdGetOrderIDFlagSuccess(t *testing.T) {
	t.Parallel()

	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/trader/v1/accounts/hash123/orders/1234567890", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"session":"NORMAL","duration":"DAY","orderType":"MARKET","orderStrategyType":"SINGLE","orderId":1234567890,"status":"FILLED"}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	configPath := writeTestConfig(t, "hash123")
	cliClient := testClient(t, server)

	// Act
	stdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "get", "--order-id", "1234567890")

	// Assert
	require.NoError(t, err)
	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	order, ok := data["order"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(1234567890), order["orderId"])
}

func TestNewOrderCmdGetOrderIDFlagWinsOverPositional(t *testing.T) {
	t.Parallel()

	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/trader/v1/accounts/hash123/orders/1234567890", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"session":"NORMAL","duration":"DAY","orderType":"MARKET","orderStrategyType":"SINGLE","orderId":1234567890,"status":"FILLED"}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	configPath := writeTestConfig(t, "hash123")
	cliClient := testClient(t, server)

	// Act
	stdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "get", "--order-id", "1234567890", "9999999")

	// Assert
	require.NoError(t, err)
	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	order, ok := data["order"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(1234567890), order["orderId"])
}

func TestNewOrderCmdGetNoAccount(t *testing.T) {
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

func TestNewOrderCmdGetMissingOrderID(t *testing.T) {
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
	assert.Contains(t, validationErr.Error(), "order-id is required")
}

func TestNewOrderCmdGetInvalidOrderID(t *testing.T) {
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

func TestNewOrderCmdCancelSuccess(t *testing.T) {
	t.Parallel()

	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete && r.URL.Path == "/trader/v1/accounts/hash123/orders/12345":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/trader/v1/accounts/hash123/orders/12345":
			writeTestOrderResponse(t, w, 12345, models.OrderStatusCanceled, "AAPL")
		default:
			assert.Failf(t, "unexpected request", "%s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	configPath := writeTestConfigMutable(t, "hash123")
	cliClient := testClient(t, server)

	// Act
	stdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "cancel", "12345")

	// Assert
	require.NoError(t, err)
	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "cancel", data["action"])
	assert.Equal(t, float64(12345), data["orderId"])
	assert.Equal(t, true, data["canceled"])
	assert.Equal(t, "verified", data["verificationState"])
	assert.Equal(t, "CANCELED", data["orderStatus"])
	order, ok := data["order"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(12345), order["orderId"])
	assert.Equal(t, "CANCELED", order["status"])
}

func TestNewOrderCmdCancelOrderIDFlagSuccess(t *testing.T) {
	t.Parallel()

	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete && r.URL.Path == "/trader/v1/accounts/hash123/orders/1234567890":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/trader/v1/accounts/hash123/orders/1234567890":
			writeTestOrderResponse(t, w, 1234567890, models.OrderStatusCanceled, "AAPL")
		default:
			assert.Failf(t, "unexpected request", "%s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	configPath := writeTestConfigMutable(t, "hash123")
	cliClient := testClient(t, server)

	// Act
	stdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "cancel", "--order-id", "1234567890")

	// Assert
	require.NoError(t, err)
	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "cancel", data["action"])
	assert.Equal(t, float64(1234567890), data["orderId"])
	assert.Equal(t, true, data["canceled"])
	assert.Equal(t, "verified", data["verificationState"])
	assert.Equal(t, "CANCELED", data["orderStatus"])
	order, ok := data["order"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(1234567890), order["orderId"])
	assert.Equal(t, "CANCELED", order["status"])
}

func TestNewOrderCmdCancelGetFailureReturnsPartialSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete && r.URL.Path == "/trader/v1/accounts/hash123/orders/12345":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/trader/v1/accounts/hash123/orders/12345":
			http.Error(w, "eventual consistency", http.StatusInternalServerError)
		default:
			assert.Failf(t, "unexpected request", "%s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	configPath := writeTestConfigMutable(t, "hash123")
	cliClient := testClient(t, server)

	stdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "cancel", "12345")
	require.NoError(t, err)

	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "cancel", data["action"])
	assert.Equal(t, float64(12345), data["orderId"])
	assert.Equal(t, true, data["canceled"])
	assert.Equal(t, "unverified", data["verificationState"])
	verificationFailures, ok := data["verificationFailures"].([]any)
	require.True(t, ok)
	require.Len(t, verificationFailures, 1)
	assert.Contains(t, verificationFailures[0], "order details unavailable")
	assert.NotContains(t, data, "order")
	require.Len(t, envelope.Errors, 1)
	assert.Contains(t, envelope.Errors[0], "order details unavailable")
}

func TestNewOrderCmdCancelMutableDisabled(t *testing.T) {
	t.Parallel()

	// Arrange
	configPath := writeTestConfig(t, "hash123")

	// Act
	stdout, err := runOrderCommand(t, nil, configPath, "", "order", "cancel", "12345")

	// Assert
	require.Error(t, err)
	assert.Empty(t, stdout)
	var validationErr *apperr.ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, mutableDisabledMessage, validationErr.Error())
}

func TestNewOrderCmdCancelAPIError(t *testing.T) {
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
	stdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "cancel", "12345")

	// Assert
	require.Error(t, err)
	assert.Empty(t, stdout)
	var httpErr *apperr.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusInternalServerError, httpErr.StatusCode)
}

func TestNewOrderCmdReplaceSuccess(t *testing.T) {
	t.Parallel()

	// Arrange
	var received models.OrderRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/trader/v1/accounts/hash123/orders/12345":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &received))

			w.Header().Set("Location", "/trader/v1/accounts/hash123/orders/67890")
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/trader/v1/accounts/hash123/orders/67890":
			writeTestOrderResponse(t, w, 67890, models.OrderStatusQueued, "AAPL")
		case r.Method == http.MethodGet && r.URL.Path == "/trader/v1/accounts/hash123/orders/12345":
			writeTestOrderResponse(t, w, 12345, models.OrderStatusReplaced, "AAPL")
		default:
			assert.Failf(t, "unexpected request", "%s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
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
	assert.Equal(t, "replace", data["action"])
	assert.Equal(t, float64(67890), data["orderId"])
	assert.Equal(t, float64(12345), data["originalOrderId"])
	assert.Equal(t, true, data["replaced"])
	assert.Equal(t, "verified", data["verificationState"])
	assert.Equal(t, "QUEUED", data["orderStatus"])
	assert.Equal(t, "REPLACED", data["originalOrderStatus"])
	submittedOrder, ok := data["submittedOrder"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "LIMIT", submittedOrder["orderType"])
	order, ok := data["order"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(67890), order["orderId"])
	assert.Equal(t, "QUEUED", order["status"])
	originalOrder, ok := data["originalOrder"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(12345), originalOrder["orderId"])
	assert.Equal(t, "REPLACED", originalOrder["status"])
}

func TestNewOrderCmdReplaceOrderIDFlagSuccess(t *testing.T) {
	t.Parallel()

	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/trader/v1/accounts/hash123/orders/1234567890":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/trader/v1/accounts/hash123/orders/1234567890":
			writeTestOrderResponse(t, w, 1234567890, models.OrderStatusReplaced, "AAPL")
		default:
			assert.Failf(t, "unexpected request", "%s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
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
		"order", "replace",
		"--order-id", "1234567890",
		"--symbol", "AAPL",
		"--action", "BUY",
		"--quantity", "10",
		"--type", "LIMIT",
		"--price", "185.25",
		"--duration", "DAY",
		"--session", "NORMAL",
	)

	// Assert
	require.NoError(t, err)
	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "replace", data["action"])
	assert.Equal(t, float64(1234567890), data["orderId"])
	assert.Equal(t, float64(1234567890), data["originalOrderId"])
	assert.Equal(t, true, data["replaced"])
	assert.Equal(t, "verified", data["verificationState"])
	assert.Equal(t, "REPLACED", data["orderStatus"])
	assert.Equal(t, "REPLACED", data["originalOrderStatus"])
	order, ok := data["order"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(1234567890), order["orderId"])
	assert.Equal(t, "REPLACED", order["status"])
	originalOrder, ok := data["originalOrder"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(1234567890), originalOrder["orderId"])
}

func TestNewOrderCmdReplaceOriginalStatusMismatchReturnsPartialSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/trader/v1/accounts/hash123/orders/12345":
			w.Header().Set("Location", "/trader/v1/accounts/hash123/orders/67890")
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/trader/v1/accounts/hash123/orders/67890":
			writeTestOrderResponse(t, w, 67890, models.OrderStatusQueued, "AAPL")
		case r.Method == http.MethodGet && r.URL.Path == "/trader/v1/accounts/hash123/orders/12345":
			writeTestOrderResponse(t, w, 12345, models.OrderStatusPendingReplace, "AAPL")
		default:
			assert.Failf(t, "unexpected request", "%s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	configPath := writeTestConfigMutable(t, "hash123")
	cliClient := testClient(t, server)

	stdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "replace", "12345", "--symbol", "AAPL", "--action", "BUY", "--quantity", "10")
	require.NoError(t, err)

	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "partial", data["verificationState"])
	assert.Equal(t, "PENDING_REPLACE", data["originalOrderStatus"])
	require.Len(t, envelope.Errors, 1)
	assert.Contains(t, envelope.Errors[0], "expected REPLACED")
}

func TestNewOrderCmdReplaceMutableDisabled(t *testing.T) {
	t.Parallel()

	// Arrange
	configPath := writeTestConfig(t, "hash123")

	// Act
	stdout, err := runOrderCommand(t, nil, configPath, "", "order", "replace", "12345", "--symbol", "AAPL", "--action", "BUY", "--quantity", "10")

	// Assert
	require.Error(t, err)
	assert.Empty(t, stdout)
	var validationErr *apperr.ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, mutableDisabledMessage, validationErr.Error())
}

func TestNewOrderCmdReplaceAPIError(t *testing.T) {
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
	stdout, err := runOrderCommand(t, cliClient, configPath, "", "order", "replace", "12345", "--symbol", "AAPL", "--action", "BUY", "--quantity", "10")

	// Assert
	require.Error(t, err)
	assert.Empty(t, stdout)
	var httpErr *apperr.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusInternalServerError, httpErr.StatusCode)
}

func TestOrderReplaceEquityFlagAliases(t *testing.T) {
	t.Parallel()

	var received models.OrderRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/trader/v1/accounts/hash123/orders/12345":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &received))

			w.Header().Set("Location", "/trader/v1/accounts/hash123/orders/67890")
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/trader/v1/accounts/hash123/orders/67890":
			writeTestOrderResponse(t, w, 67890, models.OrderStatusQueued, "AAPL")
		case r.Method == http.MethodGet && r.URL.Path == "/trader/v1/accounts/hash123/orders/12345":
			writeTestOrderResponse(t, w, 12345, models.OrderStatusReplaced, "AAPL")
		default:
			assert.Failf(t, "unexpected request", "%s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	configPath := writeTestConfigMutable(t, "hash123")
	cliClient := testClient(t, server)

	stdout, err := runOrderCommand(
		t,
		cliClient,
		configPath,
		"",
		"order", "replace", "12345",
		"--symbol", "AAPL",
		"--instruction", "BUY",
		"--quantity", "10",
		"--order-type", "LIMIT",
		"--price", "185.25",
	)

	require.NoError(t, err)
	assert.Equal(t, models.OrderTypeLimit, received.OrderType)
	require.Len(t, received.OrderLegCollection, 1)
	assert.Equal(t, models.InstructionBuy, received.OrderLegCollection[0].Instruction)
	assert.NotEmpty(t, stdout)
}

func TestOrderReplaceOptionSuccess(t *testing.T) {
	t.Parallel()

	var received models.OrderRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/trader/v1/accounts/hash123/orders/12345":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &received))

			w.Header().Set("Location", "/trader/v1/accounts/hash123/orders/67890")
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/trader/v1/accounts/hash123/orders/67890":
			writeTestOrderResponse(t, w, 67890, models.OrderStatusQueued, "AAPL  "+testFutureExpTime.Format("060102")+"C00200000")
		case r.Method == http.MethodGet && r.URL.Path == "/trader/v1/accounts/hash123/orders/12345":
			writeTestOrderResponse(t, w, 12345, models.OrderStatusReplaced, "AAPL")
		default:
			assert.Failf(t, "unexpected request", "%s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	configPath := writeTestConfigMutable(t, "hash123")
	cliClient := testClient(t, server)

	stdout, err := runOrderCommand(
		t,
		cliClient,
		configPath,
		"",
		"order", "replace", "option", "12345",
		"--underlying", "AAPL",
		"--expiration", testFutureExpDate,
		"--strike", "200",
		"--call",
		"--instruction", "BUY_TO_OPEN",
		"--quantity", "1",
		"--order-type", "LIMIT",
		"--price", "5.00",
		"--duration", "DAY",
	)

	require.NoError(t, err)
	assert.Equal(t, models.OrderTypeLimit, received.OrderType)
	require.NotNil(t, received.Price)
	assert.Equal(t, 5.00, *received.Price)
	require.Len(t, received.OrderLegCollection, 1)
	leg := received.OrderLegCollection[0]
	assert.Equal(t, models.AssetTypeOption, leg.Instrument.AssetType)
	assert.Equal(t, models.InstructionBuyToOpen, leg.Instruction)
	assert.Equal(t, "AAPL  "+testFutureExpTime.Format("060102")+"C00200000", leg.Instrument.Symbol)

	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "replace", data["action"])
	assert.Equal(t, float64(67890), data["orderId"])
	assert.Equal(t, true, data["replaced"])
}

func TestOrderReplaceOptionCallPutMutuallyExclusive(t *testing.T) {
	t.Parallel()

	configPath := writeTestConfigMutable(t, "hash123")
	stdout, err := runOrderCommand(
		t,
		nil,
		configPath,
		"",
		"order", "replace", "option", "12345",
		"--underlying", "AAPL",
		"--expiration", testFutureExpDate,
		"--strike", "200",
		"--call",
		"--put",
		"--action", "BUY_TO_OPEN",
		"--quantity", "1",
	)

	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, err.Error(), "call")
	assert.Contains(t, err.Error(), "put")
}

func TestOrderReplaceOptionMutableGuard(t *testing.T) {
	t.Parallel()

	configPath := writeTestConfig(t, "hash123")
	stdout, err := runOrderCommand(
		t,
		nil,
		configPath,
		"",
		"order", "replace", "option", "12345",
		"--underlying", "AAPL",
		"--expiration", testFutureExpDate,
		"--strike", "200",
		"--call",
		"--action", "BUY_TO_OPEN",
		"--quantity", "1",
	)

	require.Error(t, err)
	assert.Empty(t, stdout)
	var validationErr *apperr.ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, mutableDisabledMessage, validationErr.Error())
}

func TestParseRequiredOrderID(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		args      []string
		wantID    int64
		wantError string
	}{
		{name: "empty", args: []string{}, wantError: "order-id is required"},
		{name: "valid int", args: []string{"12345"}, wantID: 12345},
		{name: "flag valid int", args: []string{"--order-id", "67890"}, wantID: 67890},
		{name: "flag wins over positional", args: []string{"--order-id", "67890", "12345"}, wantID: 67890},
		{name: "invalid non-numeric", args: []string{"abc"}, wantError: "order-id must be a valid integer"},
		{name: "negative int", args: []string{"--", "-99"}, wantError: "order-id must be a positive integer"},
		{name: "zero", args: []string{"0"}, wantError: "order-id must be a positive integer"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var parsedID int64
			opts := &orderGetOpts{}
			cmd := &cobra.Command{
				Use: "order-test",
				RunE: func(cmd *cobra.Command, args []string) error {
					var err error
					parsedID, err = parseRequiredOrderID(opts.OrderID, args)
					return err
				},
			}
			cmd.Flags().StringVar(&opts.OrderID, "order-id", "", "Order ID")

			// Act
			_, err := runTestCommand(t, cmd, tc.args...)

			// Assert
			if tc.wantError == "" {
				require.NoError(t, err)
				assert.Equal(t, tc.wantID, parsedID)
				return
			}

			require.Error(t, err)
			var validationErr *apperr.ValidationError
			require.ErrorAs(t, err, &validationErr)
			assert.Contains(t, validationErr.Error(), tc.wantError)
		})
	}
}

func TestNewOrderCmdBuildOptionOutputsRequestJSON(t *testing.T) {
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

func TestNewOrderCmdBuildBracketOutputsRequestJSON(t *testing.T) {
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

func TestParseInstruction_ErrorPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"empty string", "", "action is required"},
		{"whitespace only", "   ", "action is required"},
		{"invalid action", "INVALID", "invalid action"},
		{"partial match", "BU", "invalid action"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseInstruction(tt.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)

			var validationErr *apperr.ValidationError
			require.ErrorAs(t, err, &validationErr)
		})
	}
}

func TestNewOrderCmdPreview_EmptySpec(t *testing.T) {
	// Arrange / Act
	stdout, err := runOrderCommand(t, nil, writeTestConfig(t, "hash123"), "",
		"order", "preview")

	// Assert
	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, err.Error(), "spec")
}

func TestNewOrderCmdPreview_APIError(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"server error"}`))
	}))
	defer srv.Close()

	spec := `{"session":"NORMAL","duration":"DAY","orderType":"MARKET","orderStrategyType":"SINGLE","orderLegCollection":[{"instruction":"BUY","quantity":1,"instrument":{"assetType":"EQUITY","symbol":"AAPL"}}]}`
	cliClient := &client.Ref{Client: client.NewClient("test-token", client.WithBaseURL(srv.URL))}

	// Act
	stdout, err := runOrderCommand(t, cliClient, writeTestConfig(t, "hash123"), "",
		"order", "preview", "--spec", spec)

	// Assert
	require.Error(t, err)
	assert.Empty(t, stdout)

	var httpErr *apperr.HTTPError
	require.ErrorAs(t, err, &httpErr)
}

func TestNewOrderCmdPlace_MissingSpec(t *testing.T) {
	// order place (no subcommand) requires --spec
	stdout, err := runOrderCommand(t, nil, writeTestConfigMutable(t, "hash123"), "",
		"order", "place")

	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, err.Error(), "spec")
}

func TestNewOrderCmdPlace_InvalidSpec_NonJSONPrefix(t *testing.T) {
	// Spec that doesn't start with { or [ should return the "inline JSON" error.
	stdout, err := runOrderCommand(t, nil, writeTestConfigMutable(t, "hash123"), "",
		"order", "place", "--spec", "hello world")

	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, err.Error(), "spec must be inline JSON, @file, or -")
}

func TestNewOrderCmdPlace_InvalidSpec_BadJSON(t *testing.T) {
	// Spec starts with { but isn't valid JSON.
	stdout, err := runOrderCommand(t, nil, writeTestConfigMutable(t, "hash123"), "",
		"order", "place", "--spec", "{not valid json}")

	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, err.Error(), "spec must contain valid JSON")
}

func TestNewOrderCmdPreview_SpecFileNotFound(t *testing.T) {
	stdout, err := runOrderCommand(t, nil, writeTestConfig(t, "hash123"), "",
		"order", "preview", "--spec", "@/nonexistent/file.json")

	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, err.Error(), "read spec file")
}

func TestNewOrderCmdReplace_MissingOrderID(t *testing.T) {
	// Replace requires a positional order-id argument.
	stdout, err := runOrderCommand(t, nil, writeTestConfigMutable(t, "hash123"), "",
		"order", "replace",
		"--symbol", "AAPL", "--action", "BUY", "--quantity", "10")

	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, err.Error(), "order-id is required")
}

func TestNewOrderCmdReplace_InvalidAction(t *testing.T) {
	// Replace parses equity params; invalid --action triggers parseInstruction error.
	stdout, err := runOrderCommand(t, nil, writeTestConfigMutable(t, "hash123"), "",
		"order", "replace", "12345",
		"--symbol", "AAPL", "--action", "INVALID", "--quantity", "10")

	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, err.Error(), "invalid action")
}

// TestNewOrderCmdReplaceInfersLimitFromPrice verifies that when --price is
// provided without --type, order replace infers LIMIT instead of defaulting to
// MARKET. This is the fix for the bug where chasing a limit order with a new
// price silently submitted a MARKET order.
func TestNewOrderCmdReplaceInfersLimitFromPrice(t *testing.T) {
	t.Parallel()

	// Arrange
	var received models.OrderRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/trader/v1/accounts/hash123/orders/12345":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &received))

			w.Header().Set("Location", "/trader/v1/accounts/hash123/orders/67890")
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/trader/v1/accounts/hash123/orders/67890":
			writeTestOrderResponse(t, w, 67890, models.OrderStatusQueued, "CRDO")
		case r.Method == http.MethodGet && r.URL.Path == "/trader/v1/accounts/hash123/orders/12345":
			writeTestOrderResponse(t, w, 12345, models.OrderStatusReplaced, "CRDO")
		default:
			assert.Failf(t, "unexpected request", "%s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	configPath := writeTestConfigMutable(t, "hash123")
	cliClient := testClient(t, server)

	// Act: pass --price but omit --type; should infer LIMIT
	stdout, err := runOrderCommand(
		t,
		cliClient,
		configPath,
		"",
		"order", "replace", "12345",
		"--symbol", "CRDO",
		"--action", "BUY",
		"--quantity", "68",
		"--price", "194.16",
	)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, models.OrderTypeLimit, received.OrderType)
	require.NotNil(t, received.Price)
	assert.Equal(t, 194.16, *received.Price)

	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	submittedOrder, ok := data["submittedOrder"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "LIMIT", submittedOrder["orderType"])
}

// TestOrderReplaceOptionInfersLimitFromPrice verifies that order replace option
// infers LIMIT when --price is provided but --type/--order-type is omitted.
func TestOrderReplaceOptionInfersLimitFromPrice(t *testing.T) {
	t.Parallel()

	// Arrange
	var received models.OrderRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/trader/v1/accounts/hash123/orders/12345":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &received))

			w.Header().Set("Location", "/trader/v1/accounts/hash123/orders/67890")
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/trader/v1/accounts/hash123/orders/67890":
			writeTestOrderResponse(t, w, 67890, models.OrderStatusQueued, "AAPL  "+testFutureExpTime.Format("060102")+"C00200000")
		case r.Method == http.MethodGet && r.URL.Path == "/trader/v1/accounts/hash123/orders/12345":
			writeTestOrderResponse(t, w, 12345, models.OrderStatusReplaced, "AAPL")
		default:
			assert.Failf(t, "unexpected request", "%s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	configPath := writeTestConfigMutable(t, "hash123")
	cliClient := testClient(t, server)

	// Act: pass --price but omit --type/--order-type; should infer LIMIT
	stdout, err := runOrderCommand(
		t,
		cliClient,
		configPath,
		"",
		"order", "replace", "option", "12345",
		"--underlying", "AAPL",
		"--expiration", testFutureExpDate,
		"--strike", "200",
		"--call",
		"--action", "BUY_TO_OPEN",
		"--quantity", "1",
		"--price", "6.50",
	)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, models.OrderTypeLimit, received.OrderType)
	require.NotNil(t, received.Price)
	assert.Equal(t, 6.50, *received.Price)

	envelope := decodeEnvelope(t, stdout)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "replace", data["action"])
	submittedOrder, ok := data["submittedOrder"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "LIMIT", submittedOrder["orderType"])
}

// --- FTS build tests ---

// validPrimarySpec returns a minimal valid primary order spec for FTS tests.
func validPrimarySpec(t *testing.T) string {
	t.Helper()
	price := 150.0

	return mustMarshalJSON(t, models.OrderRequest{
		Session:           models.SessionNormal,
		Duration:          models.DurationDay,
		OrderType:         models.OrderTypeLimit,
		OrderStrategyType: models.OrderStrategyTypeSingle,
		Price:             &price,
		OrderLegCollection: []models.OrderLegCollection{
			{
				Instruction: models.InstructionBuy,
				Quantity:    10,
				Instrument:  models.OrderInstrument{AssetType: models.AssetTypeEquity, Symbol: "AAPL"},
			},
		},
	})
}

// validSecondarySpec returns a minimal valid secondary order spec for FTS tests.
func validSecondarySpec(t *testing.T) string {
	t.Helper()
	price := 160.0

	return mustMarshalJSON(t, models.OrderRequest{
		Session:           models.SessionNormal,
		Duration:          models.DurationDay,
		OrderType:         models.OrderTypeLimit,
		OrderStrategyType: models.OrderStrategyTypeSingle,
		Price:             &price,
		OrderLegCollection: []models.OrderLegCollection{
			{
				Instruction: models.InstructionSell,
				Quantity:    10,
				Instrument:  models.OrderInstrument{AssetType: models.AssetTypeEquity, Symbol: "AAPL"},
			},
		},
	})
}

func TestNewOrderCmdBuildFTSInlineJSON(t *testing.T) {
	t.Parallel()

	primary := validPrimarySpec(t)
	secondary := validSecondarySpec(t)

	stdout, err := runOrderCommand(t, nil, writeTestConfig(t, "hash123"), "",
		"order", "build", "fts",
		"--primary", primary,
		"--secondary", secondary,
	)
	require.NoError(t, err)

	// Arrange: decode the output
	order := decodeOrderRequest(t, stdout)

	// Assert: primary becomes TRIGGER
	assert.Equal(t, models.OrderStrategyTypeTrigger, order.OrderStrategyType)
	assert.Equal(t, models.OrderTypeLimit, order.OrderType)
	require.NotNil(t, order.Price)
	assert.Equal(t, 150.0, *order.Price)
	require.Len(t, order.OrderLegCollection, 1)
	assert.Equal(t, "AAPL", order.OrderLegCollection[0].Instrument.Symbol)
	assert.Equal(t, models.InstructionBuy, order.OrderLegCollection[0].Instruction)

	// Assert: secondary becomes SINGLE child
	require.Len(t, order.ChildOrderStrategies, 1)
	child := order.ChildOrderStrategies[0]
	assert.Equal(t, models.OrderStrategyTypeSingle, child.OrderStrategyType)
	assert.Equal(t, models.OrderTypeLimit, child.OrderType)
	require.NotNil(t, child.Price)
	assert.Equal(t, 160.0, *child.Price)
	require.Len(t, child.OrderLegCollection, 1)
	assert.Equal(t, models.InstructionSell, child.OrderLegCollection[0].Instruction)
}

func TestNewOrderCmdBuildFTSFromFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	primaryPath := filepath.Join(tmpDir, "primary.json")
	require.NoError(t, os.WriteFile(primaryPath, []byte(validPrimarySpec(t)), 0o600))

	secondaryPath := filepath.Join(tmpDir, "secondary.json")
	require.NoError(t, os.WriteFile(secondaryPath, []byte(validSecondarySpec(t)), 0o600))

	stdout, err := runOrderCommand(t, nil, writeTestConfig(t, "hash123"), "",
		"order", "build", "fts",
		"--primary", "@"+primaryPath,
		"--secondary", "@"+secondaryPath,
	)
	require.NoError(t, err)

	order := decodeOrderRequest(t, stdout)
	assert.Equal(t, models.OrderStrategyTypeTrigger, order.OrderStrategyType)
	require.Len(t, order.ChildOrderStrategies, 1)
	assert.Equal(t, models.OrderStrategyTypeSingle, order.ChildOrderStrategies[0].OrderStrategyType)
}

func TestNewOrderCmdBuildFTSMissingPrimary(t *testing.T) {
	t.Parallel()

	// Cobra validates required flags before RunE executes,
	// so a missing --primary should produce an error.
	_, err := runOrderCommand(t, nil, writeTestConfig(t, "hash123"), "",
		"order", "build", "fts",
		"--secondary", validSecondarySpec(t),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "primary")
}

func TestNewOrderCmdBuildFTSMissingSecondary(t *testing.T) {
	t.Parallel()

	_, err := runOrderCommand(t, nil, writeTestConfig(t, "hash123"), "",
		"order", "build", "fts",
		"--primary", validPrimarySpec(t),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "secondary")
}

func TestNewOrderCmdBuildFTSValidationError(t *testing.T) {
	t.Parallel()

	// An empty order ({}) has no OrderType and no legs, so ValidateFTSOrder
	// should reject it as "primary order is empty".
	emptySpec := `{}`

	_, err := runOrderCommand(t, nil, writeTestConfig(t, "hash123"), "",
		"order", "build", "fts",
		"--primary", emptySpec,
		"--secondary", validSecondarySpec(t),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "primary order is empty")
}

func TestNewOrderCmdBuildFTSInvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := runOrderCommand(t, nil, writeTestConfig(t, "hash123"), "",
		"order", "build", "fts",
		"--primary", `{not valid json}`,
		"--secondary", validSecondarySpec(t),
	)
	require.Error(t, err)
	// readSpecSource rejects invalid JSON before we even try to unmarshal
	assert.Contains(t, err.Error(), "valid JSON")
}

func TestNewOrderCmdBuildFTSRejectsComplexPrimary(t *testing.T) {
	t.Parallel()
	price := 150.0

	// A primary with OrderStrategyType=TRIGGER should be rejected by
	// ValidateFTSOrder since FTS legs cannot be nested complex orders.
	triggerSpec := mustMarshalJSON(t, models.OrderRequest{
		Session:           models.SessionNormal,
		Duration:          models.DurationDay,
		OrderType:         models.OrderTypeLimit,
		OrderStrategyType: models.OrderStrategyTypeTrigger,
		Price:             &price,
		OrderLegCollection: []models.OrderLegCollection{
			{
				Instruction: models.InstructionBuy,
				Quantity:    10,
				Instrument:  models.OrderInstrument{AssetType: models.AssetTypeEquity, Symbol: "AAPL"},
			},
		},
	})

	_, err := runOrderCommand(t, nil, writeTestConfig(t, "hash123"), "",
		"order", "build", "fts",
		"--primary", triggerSpec,
		"--secondary", validSecondarySpec(t),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "simple order")
}

// TestParseOrderType verifies order type parsing including MOC/LOC aliases.
func TestParseOrderType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		fallback models.OrderType
		want     models.OrderType
		wantErr  bool
	}{
		{
			name:     "MARKET_ON_CLOSE full name",
			input:    "MARKET_ON_CLOSE",
			fallback: models.OrderTypeMarket,
			want:     models.OrderTypeMarketOnClose,
			wantErr:  false,
		},
		{
			name:     "MOC alias",
			input:    "MOC",
			fallback: models.OrderTypeMarket,
			want:     models.OrderTypeMarketOnClose,
			wantErr:  false,
		},
		{
			name:     "LIMIT_ON_CLOSE full name",
			input:    "LIMIT_ON_CLOSE",
			fallback: models.OrderTypeMarket,
			want:     models.OrderTypeLimitOnClose,
			wantErr:  false,
		},
		{
			name:     "LOC alias",
			input:    "LOC",
			fallback: models.OrderTypeMarket,
			want:     models.OrderTypeLimitOnClose,
			wantErr:  false,
		},
		{
			name:     "MARKET",
			input:    "MARKET",
			fallback: models.OrderTypeLimit,
			want:     models.OrderTypeMarket,
			wantErr:  false,
		},
		{
			name:     "LIMIT",
			input:    "LIMIT",
			fallback: models.OrderTypeMarket,
			want:     models.OrderTypeLimit,
			wantErr:  false,
		},
		{
			name:     "STOP",
			input:    "STOP",
			fallback: models.OrderTypeMarket,
			want:     models.OrderTypeStop,
			wantErr:  false,
		},
		{
			name:     "STOP_LIMIT",
			input:    "STOP_LIMIT",
			fallback: models.OrderTypeMarket,
			want:     models.OrderTypeStopLimit,
			wantErr:  false,
		},
		{
			name:     "empty string uses fallback",
			input:    "",
			fallback: models.OrderTypeMarket,
			want:     models.OrderTypeMarket,
			wantErr:  false,
		},
		{
			name:     "whitespace uses fallback",
			input:    "   ",
			fallback: models.OrderTypeLimit,
			want:     models.OrderTypeLimit,
			wantErr:  false,
		},
		{
			name:     "case insensitive",
			input:    "market",
			fallback: models.OrderTypeLimit,
			want:     models.OrderTypeMarket,
			wantErr:  false,
		},
		{
			name:     "invalid type",
			input:    "INVALID_TYPE",
			fallback: models.OrderTypeMarket,
			want:     "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseOrderType(tt.input, tt.fallback)
			if tt.wantErr {
				require.Error(t, err)
				_, ok := errors.AsType[*apperr.ValidationError](err)
				require.True(t, ok)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// TestParseDuration verifies duration parsing including GTC/FOK/IOC aliases.
func TestParseDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    models.Duration
		wantErr bool
	}{
		{
			name:    "DAY full name",
			input:   "DAY",
			want:    models.DurationDay,
			wantErr: false,
		},
		{
			name:    "GOOD_TILL_CANCEL full name",
			input:   "GOOD_TILL_CANCEL",
			want:    models.DurationGoodTillCancel,
			wantErr: false,
		},
		{
			name:    "GTC alias",
			input:   "GTC",
			want:    models.DurationGoodTillCancel,
			wantErr: false,
		},
		{
			name:    "FILL_OR_KILL full name",
			input:   "FILL_OR_KILL",
			want:    models.DurationFillOrKill,
			wantErr: false,
		},
		{
			name:    "FOK alias",
			input:   "FOK",
			want:    models.DurationFillOrKill,
			wantErr: false,
		},
		{
			name:    "IMMEDIATE_OR_CANCEL full name",
			input:   "IMMEDIATE_OR_CANCEL",
			want:    models.DurationImmediateOrCancel,
			wantErr: false,
		},
		{
			name:    "IOC alias",
			input:   "IOC",
			want:    models.DurationImmediateOrCancel,
			wantErr: false,
		},
		{
			name:    "case insensitive alias",
			input:   "gtc",
			want:    models.DurationGoodTillCancel,
			wantErr: false,
		},
		{
			name:    "empty string returns empty default",
			input:   "",
			want:    "",
			wantErr: false,
		},
		{
			name:    "whitespace returns empty default",
			input:   "   ",
			want:    "",
			wantErr: false,
		},
		{
			name:    "invalid duration",
			input:   "FOREVER",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseDuration(tt.input)
			if tt.wantErr {
				require.Error(t, err)

				_, ok := errors.AsType[*apperr.ValidationError](err)
				require.True(t, ok)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// farExpDate is testFutureExpTime + 30 days, formatted as YYYY-MM-DD.
// Calendar and diagonal spreads need two different expirations.
var farExpDate = testFutureExpTime.AddDate(0, 0, 30).Format("2006-01-02")

func TestNewOrderCmdBuildCalendarCallOpen(t *testing.T) {
	t.Parallel()

	// Long calendar call spread: buy far-dated call, sell near-dated call.
	stdout, err := runOrderCommand(
		t, nil, writeTestConfig(t, "hash123"), "",
		"order", "build", "calendar",
		"--underlying", "AAPL",
		"--near-expiration", testFutureExpDate,
		"--far-expiration", farExpDate,
		"--strike", "150",
		"--call",
		"--open",
		"--quantity", "1",
		"--price", "2.50",
		"--duration", "DAY",
	)
	require.NoError(t, err)

	order := decodeOrderRequest(t, stdout)
	assert.Equal(t, models.OrderTypeNetDebit, order.OrderType)
	require.NotNil(t, order.ComplexOrderStrategyType)
	assert.Equal(t, models.ComplexOrderStrategyTypeCalendar, *order.ComplexOrderStrategyType)
	require.NotNil(t, order.Price)
	assert.Equal(t, 2.50, *order.Price)
	assert.Equal(t, models.DurationDay, order.Duration)

	require.Len(t, order.OrderLegCollection, 2)

	// Leg 0: far-dated call (bought).
	assert.Equal(t, models.InstructionBuyToOpen, order.OrderLegCollection[0].Instruction)
	require.NotNil(t, order.OrderLegCollection[0].Instrument.PutCall)
	assert.Equal(t, models.PutCallCall, *order.OrderLegCollection[0].Instrument.PutCall)
	assert.Equal(t, 1.0, order.OrderLegCollection[0].Quantity)

	// Leg 1: near-dated call (sold).
	assert.Equal(t, models.InstructionSellToOpen, order.OrderLegCollection[1].Instruction)
	require.NotNil(t, order.OrderLegCollection[1].Instrument.PutCall)
	assert.Equal(t, models.PutCallCall, *order.OrderLegCollection[1].Instrument.PutCall)
}

func TestNewOrderCmdBuildCalendarPutClose(t *testing.T) {
	t.Parallel()

	// Closing a put calendar: sell far-dated put, buy near-dated put.
	stdout, err := runOrderCommand(
		t, nil, writeTestConfig(t, "hash123"), "",
		"order", "build", "calendar",
		"--underlying", "AAPL",
		"--near-expiration", testFutureExpDate,
		"--far-expiration", farExpDate,
		"--strike", "150",
		"--put",
		"--close",
		"--quantity", "1",
		"--price", "2.00",
	)
	require.NoError(t, err)

	order := decodeOrderRequest(t, stdout)
	require.Len(t, order.OrderLegCollection, 2)

	// Closing reverses instructions: far leg sells, near leg buys, NET_CREDIT.
	assert.Equal(t, models.OrderTypeNetCredit, order.OrderType)
	assert.Equal(t, models.InstructionSellToClose, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, models.InstructionBuyToClose, order.OrderLegCollection[1].Instruction)

	// Both legs should be puts.
	for _, leg := range order.OrderLegCollection {
		require.NotNil(t, leg.Instrument.PutCall)
		assert.Equal(t, models.PutCallPut, *leg.Instrument.PutCall)
	}
}

func TestNewOrderCmdBuildDiagonalCallOpen(t *testing.T) {
	t.Parallel()

	// Diagonal call: different strikes AND different expirations.
	stdout, err := runOrderCommand(
		t, nil, writeTestConfig(t, "hash123"), "",
		"order", "build", "diagonal",
		"--underlying", "AAPL",
		"--near-expiration", testFutureExpDate,
		"--far-expiration", farExpDate,
		"--near-strike", "150",
		"--far-strike", "160",
		"--call",
		"--open",
		"--quantity", "2",
		"--price", "3.00",
	)
	require.NoError(t, err)

	order := decodeOrderRequest(t, stdout)
	assert.Equal(t, models.OrderTypeNetDebit, order.OrderType)
	require.NotNil(t, order.ComplexOrderStrategyType)
	assert.Equal(t, models.ComplexOrderStrategyTypeDiagonal, *order.ComplexOrderStrategyType)
	require.NotNil(t, order.Price)
	assert.Equal(t, 3.00, *order.Price)

	require.Len(t, order.OrderLegCollection, 2)

	// Leg 0: far-dated call at 160 (bought).
	assert.Equal(t, models.InstructionBuyToOpen, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, 2.0, order.OrderLegCollection[0].Quantity)
	require.NotNil(t, order.OrderLegCollection[0].Instrument.OptionStrikePrice)
	assert.Equal(t, 160.0, *order.OrderLegCollection[0].Instrument.OptionStrikePrice)
	require.NotNil(t, order.OrderLegCollection[0].Instrument.PutCall)
	assert.Equal(t, models.PutCallCall, *order.OrderLegCollection[0].Instrument.PutCall)

	// Leg 1: near-dated call at 150 (sold).
	assert.Equal(t, models.InstructionSellToOpen, order.OrderLegCollection[1].Instruction)
	require.NotNil(t, order.OrderLegCollection[1].Instrument.OptionStrikePrice)
	assert.Equal(t, 150.0, *order.OrderLegCollection[1].Instrument.OptionStrikePrice)
}

func TestNewOrderCmdBuildDiagonalPutOpen(t *testing.T) {
	t.Parallel()

	stdout, err := runOrderCommand(
		t, nil, writeTestConfig(t, "hash123"), "",
		"order", "build", "diagonal",
		"--underlying", "AAPL",
		"--near-expiration", testFutureExpDate,
		"--far-expiration", farExpDate,
		"--near-strike", "150",
		"--far-strike", "140",
		"--put",
		"--open",
		"--quantity", "1",
		"--price", "2.50",
	)
	require.NoError(t, err)

	order := decodeOrderRequest(t, stdout)
	require.Len(t, order.OrderLegCollection, 2)

	// Both legs should be puts.
	for _, leg := range order.OrderLegCollection {
		require.NotNil(t, leg.Instrument.PutCall)
		assert.Equal(t, models.PutCallPut, *leg.Instrument.PutCall)
	}
}

func TestNewOrderCmdBuildCollarOpen(t *testing.T) {
	t.Parallel()

	// Collar opening: buy shares, buy protective put, sell covered call.
	stdout, err := runOrderCommand(
		t, nil, writeTestConfig(t, "hash123"), "",
		"order", "build", "collar",
		"--underlying", "AAPL",
		"--put-strike", "140",
		"--call-strike", "160",
		"--expiration", testFutureExpDate,
		"--quantity", "3",
		"--price", "450",
		"--open",
	)
	require.NoError(t, err)

	order := decodeOrderRequest(t, stdout)
	assert.Equal(t, models.OrderTypeNetDebit, order.OrderType)
	require.NotNil(t, order.ComplexOrderStrategyType)
	assert.Equal(t, models.ComplexOrderStrategyTypeCollarWithStock, *order.ComplexOrderStrategyType)
	require.NotNil(t, order.Price)
	assert.Equal(t, 450.0, *order.Price)

	// 3 legs: equity, protective put, covered call.
	require.Len(t, order.OrderLegCollection, 3)

	// Leg 0: equity BUY, 300 shares (3 contracts * 100).
	equityLeg := order.OrderLegCollection[0]
	assert.Equal(t, models.InstructionBuy, equityLeg.Instruction)
	assert.Equal(t, 300.0, equityLeg.Quantity)
	assert.Equal(t, models.AssetTypeEquity, equityLeg.Instrument.AssetType)

	// Leg 1: protective put BUY_TO_OPEN.
	putLeg := order.OrderLegCollection[1]
	assert.Equal(t, models.InstructionBuyToOpen, putLeg.Instruction)
	require.NotNil(t, putLeg.Instrument.PutCall)
	assert.Equal(t, models.PutCallPut, *putLeg.Instrument.PutCall)
	require.NotNil(t, putLeg.Instrument.OptionStrikePrice)
	assert.Equal(t, 140.0, *putLeg.Instrument.OptionStrikePrice)

	// Leg 2: covered call SELL_TO_OPEN.
	callLeg := order.OrderLegCollection[2]
	assert.Equal(t, models.InstructionSellToOpen, callLeg.Instruction)
	require.NotNil(t, callLeg.Instrument.PutCall)
	assert.Equal(t, models.PutCallCall, *callLeg.Instrument.PutCall)
	require.NotNil(t, callLeg.Instrument.OptionStrikePrice)
	assert.Equal(t, 160.0, *callLeg.Instrument.OptionStrikePrice)
}

func TestNewOrderCmdBuildCollarClose(t *testing.T) {
	t.Parallel()

	// Collar closing: sell shares, sell put, buy call.
	stdout, err := runOrderCommand(
		t, nil, writeTestConfig(t, "hash123"), "",
		"order", "build", "collar",
		"--underlying", "AAPL",
		"--put-strike", "140",
		"--call-strike", "160",
		"--expiration", testFutureExpDate,
		"--quantity", "2",
		"--price", "300",
		"--close",
	)
	require.NoError(t, err)

	order := decodeOrderRequest(t, stdout)
	require.Len(t, order.OrderLegCollection, 3)

	// Closing reverses all instructions, NET_CREDIT.
	assert.Equal(t, models.OrderTypeNetCredit, order.OrderType)
	assert.Equal(t, models.InstructionSell, order.OrderLegCollection[0].Instruction)
	assert.Equal(t, models.InstructionSellToClose, order.OrderLegCollection[1].Instruction)
	assert.Equal(t, models.InstructionBuyToClose, order.OrderLegCollection[2].Instruction)
}

func TestNewOrderCmdBuildCalendarInvalidDate(t *testing.T) {
	t.Parallel()

	// Bad near-expiration format should produce a parseDateFlag error.
	_, err := runOrderCommand(
		t, nil, writeTestConfig(t, "hash123"), "",
		"order", "build", "calendar",
		"--underlying", "AAPL",
		"--near-expiration", "not-a-date",
		"--far-expiration", farExpDate,
		"--strike", "150",
		"--call",
		"--open",
		"--quantity", "1",
		"--price", "2.50",
	)
	require.Error(t, err)

	var validationErr *apperr.ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Contains(t, validationErr.Error(), "near-expiration must use YYYY-MM-DD format")
}

func TestNewOrderCmdBuildDiagonalInvalidDate(t *testing.T) {
	t.Parallel()

	// Bad far-expiration format should produce a parseDateFlag error.
	_, err := runOrderCommand(
		t, nil, writeTestConfig(t, "hash123"), "",
		"order", "build", "diagonal",
		"--underlying", "AAPL",
		"--near-expiration", testFutureExpDate,
		"--far-expiration", "bad-date",
		"--near-strike", "150",
		"--far-strike", "160",
		"--call",
		"--open",
		"--quantity", "1",
		"--price", "3.00",
	)
	require.Error(t, err)

	var validationErr *apperr.ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Contains(t, validationErr.Error(), "far-expiration must use YYYY-MM-DD format")
}
