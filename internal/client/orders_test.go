package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/models"
)

// testOrderRequest creates a simple test order request fixture.
func testOrderRequest() *models.OrderRequest {
	qty := 10.0
	return &models.OrderRequest{
		Session:           models.SessionNormal,
		Duration:          models.DurationDay,
		OrderType:         models.OrderTypeMarket,
		OrderStrategyType: models.OrderStrategyTypeSingle,
		Quantity:          &qty,
		OrderLegCollection: []models.OrderLegCollection{
			{
				Instrument: models.OrderInstrument{
					AssetType: models.AssetTypeEquity,
					Symbol:    "AAPL",
				},
				Instruction: models.InstructionBuy,
				Quantity:    10,
			},
		},
	}
}

func TestListOrders_Success(t *testing.T) {
	orderID := int64(12345)
	status := models.OrderStatusFilled

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/trader/v1/accounts/abc123/orders", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		response := []models.Order{
			{
				Session:           models.SessionNormal,
				Duration:          models.DurationDay,
				OrderType:         models.OrderTypeMarket,
				OrderStrategyType: models.OrderStrategyTypeSingle,
				OrderID:           &orderID,
				Status:            &status,
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.ListOrders(context.Background(), "abc123", OrderListParams{})

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, int64(12345), *result[0].OrderID)
	assert.Equal(t, models.OrderStatusFilled, *result[0].Status)
}

func TestListOrders_WithParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "FILLED", r.URL.Query().Get("status"))
		assert.Equal(t, "2024-01-01T00:00:00.000Z", r.URL.Query().Get("fromEnteredTime"))
		assert.Equal(t, "2024-12-31T23:59:59.000Z", r.URL.Query().Get("toEnteredTime"))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode([]models.Order{}))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.ListOrders(context.Background(), "abc123", OrderListParams{
		Status:          "FILLED",
		FromEnteredTime: "2024-01-01T00:00:00.000Z",
		ToEnteredTime:   "2024-12-31T23:59:59.000Z",
	})

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestAllOrders_Success(t *testing.T) {
	orderID := int64(99999)
	status := models.OrderStatusWorking

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/trader/v1/orders", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		response := []models.Order{
			{
				Session:           models.SessionNormal,
				Duration:          models.DurationDay,
				OrderType:         models.OrderTypeLimit,
				OrderStrategyType: models.OrderStrategyTypeSingle,
				OrderID:           &orderID,
				Status:            &status,
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.AllOrders(context.Background(), OrderListParams{})

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, int64(99999), *result[0].OrderID)
	assert.Equal(t, models.OrderStatusWorking, *result[0].Status)
}

func TestGetOrder_Success(t *testing.T) {
	orderID := int64(12345)
	status := models.OrderStatusFilled

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/trader/v1/accounts/abc123/orders/12345", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		response := models.Order{
			Session:           models.SessionNormal,
			Duration:          models.DurationDay,
			OrderType:         models.OrderTypeMarket,
			OrderStrategyType: models.OrderStrategyTypeSingle,
			OrderID:           &orderID,
			Status:            &status,
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.GetOrder(context.Background(), "abc123", 12345)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(12345), *result.OrderID)
	assert.Equal(t, models.OrderStatusFilled, *result.Status)
}

func TestPlaceOrder_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/trader/v1/accounts/abc123/orders", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Verify request body is correctly serialized.
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var req models.OrderRequest
		require.NoError(t, json.Unmarshal(body, &req))
		assert.Equal(t, models.OrderTypeMarket, req.OrderType)
		assert.Equal(t, models.SessionNormal, req.Session)
		require.Len(t, req.OrderLegCollection, 1)
		assert.Equal(t, "AAPL", req.OrderLegCollection[0].Instrument.Symbol)
		assert.Equal(t, models.InstructionBuy, req.OrderLegCollection[0].Instruction)

		w.Header().Set("Location", "/trader/v1/accounts/abc123/orders/67890")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.PlaceOrder(context.Background(), "abc123", testOrderRequest())

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(67890), result.OrderID)
}

func TestPlaceOrder_400_ReturnsOrderRejectedError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"Validation failed","errors":[{"detail":"Invalid order"}]}`))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.PlaceOrder(context.Background(), "abc123", testOrderRequest())

	require.Error(t, err)
	assert.Nil(t, result)

	var orderErr *apperr.OrderRejectedError
	require.ErrorAs(t, err, &orderErr)
	assert.Contains(t, orderErr.Error(), "order rejected")
}

func TestPlaceOrder_422_ReturnsOrderRejectedError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"Unprocessable Entity"}`))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.PlaceOrder(context.Background(), "abc123", testOrderRequest())

	require.Error(t, err)
	assert.Nil(t, result)

	var orderErr *apperr.OrderRejectedError
	require.ErrorAs(t, err, &orderErr)
	assert.Contains(t, orderErr.Error(), "order rejected")
}

func TestPreviewOrder_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/trader/v1/accounts/abc123/previewOrder", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Verify request body matches the order format.
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var req models.OrderRequest
		require.NoError(t, json.Unmarshal(body, &req))
		assert.Equal(t, models.OrderTypeMarket, req.OrderType)

		w.Header().Set("Content-Type", "application/json")
		commValue := 0.65
		response := models.PreviewOrder{
			OrderID: ptr(int64(11111)),
			CommissionAndFee: &models.CommissionAndFee{
				Commission: &models.CommissionDetail{
					CommissionLegs: []models.CommissionLeg{
						{CommissionValues: []models.CommissionValue{
							{Value: &commValue, Type: "COMMISSION"},
						}},
					},
				},
			},
		}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.PreviewOrder(context.Background(), "abc123", testOrderRequest())

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(11111), *result.OrderID)
	require.NotNil(t, result.CommissionAndFee.Commission)
	require.Len(t, result.CommissionAndFee.Commission.CommissionLegs, 1)
	require.Len(t, result.CommissionAndFee.Commission.CommissionLegs[0].CommissionValues, 1)
	assert.Equal(t, 0.65, *result.CommissionAndFee.Commission.CommissionLegs[0].CommissionValues[0].Value)
}

func TestReplaceOrder_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/trader/v1/accounts/abc123/orders/12345", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Verify the replacement body is sent.
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var req models.OrderRequest
		require.NoError(t, json.Unmarshal(body, &req))
		assert.Equal(t, models.OrderTypeMarket, req.OrderType)
		require.Len(t, req.OrderLegCollection, 1)
		assert.Equal(t, "AAPL", req.OrderLegCollection[0].Instrument.Symbol)

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	err := c.ReplaceOrder(context.Background(), "abc123", 12345, testOrderRequest())

	require.NoError(t, err)
}

func TestCancelOrder_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/trader/v1/accounts/abc123/orders/12345", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	err := c.CancelOrder(context.Background(), "abc123", 12345)

	require.NoError(t, err)
}
