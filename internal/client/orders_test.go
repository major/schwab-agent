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
		Statuses:        []string{"FILLED"},
		FromEnteredTime: "2024-01-01T00:00:00.000Z",
		ToEnteredTime:   "2024-12-31T23:59:59.000Z",
	})

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestListOrders_MultipleStatuses(t *testing.T) {
	// The Schwab API accepts only a single status per request, so multiple
	// statuses require separate API calls with merged results.
	workingID := int64(111)
	filledID := int64(222)
	workingStatus := models.OrderStatusWorking
	filledStatus := models.OrderStatusFilled

	var requestCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		status := r.URL.Query().Get("status")

		w.Header().Set("Content-Type", "application/json")
		switch status {
		case "WORKING":
			response := []models.Order{{
				OrderID:           &workingID,
				Status:            &workingStatus,
				OrderType:         models.OrderTypeLimit,
				OrderStrategyType: models.OrderStrategyTypeSingle,
			}}
			require.NoError(t, json.NewEncoder(w).Encode(response))
		case "FILLED":
			response := []models.Order{{
				OrderID:           &filledID,
				Status:            &filledStatus,
				OrderType:         models.OrderTypeMarket,
				OrderStrategyType: models.OrderStrategyTypeSingle,
			}}
			require.NoError(t, json.NewEncoder(w).Encode(response))
		default:
			t.Errorf("unexpected status filter: %q", status)
		}
	}))
	defer srv.Close()

	// Act
	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.ListOrders(context.Background(), "abc123", OrderListParams{
		Statuses: []string{"WORKING", "FILLED"},
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 2, requestCount, "should make one API call per status")
	require.Len(t, result, 2)
	assert.Equal(t, int64(111), *result[0].OrderID)
	assert.Equal(t, int64(222), *result[1].OrderID)
}

func TestListOrders_MultipleStatuses_Dedup(t *testing.T) {
	// Guard against the unlikely case where the same order appears in multiple
	// status responses (e.g. status transition mid-request).
	orderID := int64(333)
	workingStatus := models.OrderStatusWorking

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := []models.Order{{
			OrderID:           &orderID,
			Status:            &workingStatus,
			OrderType:         models.OrderTypeLimit,
			OrderStrategyType: models.OrderStrategyTypeSingle,
		}}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer srv.Close()

	// Act
	c := NewClient("test-token", WithBaseURL(srv.URL))
	result, err := c.ListOrders(context.Background(), "abc123", OrderListParams{
		Statuses: []string{"WORKING", "PENDING_ACTIVATION"},
	})

	// Assert - should dedup the duplicate OrderID.
	require.NoError(t, err)
	require.Len(t, result, 1, "duplicate OrderID should be deduplicated")
	assert.Equal(t, int64(333), *result[0].OrderID)
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

func TestOrderIDFromLocation(t *testing.T) {
	tests := []struct {
		name     string
		location string
		want     int64
	}{
		{
			name:     "relative path",
			location: "/trader/v1/accounts/abc123/orders/67890",
			want:     67890,
		},
		{
			name:     "absolute URL",
			location: "https://api.schwabapi.com/trader/v1/accounts/abc123/orders/67890",
			want:     67890,
		},
		{
			name:     "trailing slash",
			location: "/trader/v1/accounts/abc123/orders/67890/",
			want:     67890,
		},
		{
			name:     "query string",
			location: "/trader/v1/accounts/abc123/orders/67890?source=proxy",
			want:     67890,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := orderIDFromLocation(tt.location)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestOrderIDFromLocation_MissingOrderID(t *testing.T) {
	_, err := orderIDFromLocation("https://api.schwabapi.com/")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing order ID")
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
			OrderID: new(int64(11111)),
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

		w.Header().Set("Location", "/trader/v1/accounts/abc123/orders/67890")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	resp, err := c.ReplaceOrder(context.Background(), "abc123", 12345, testOrderRequest())

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int64(67890), resp.OrderID)
}

func TestReplaceOrder_NoLocationHeaderFallsBackToOriginalOrderID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/trader/v1/accounts/abc123/orders/12345", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	resp, err := c.ReplaceOrder(context.Background(), "abc123", 12345, testOrderRequest())

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int64(12345), resp.OrderID)
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

func TestPlaceOrder_401_ReturnsAuthExpiredError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	_, err := c.PlaceOrder(context.Background(), "abc123", testOrderRequest())

	require.Error(t, err)

	var authErr *apperr.AuthExpiredError
	require.ErrorAs(t, err, &authErr)
}

func TestPlaceOrder_500_ReturnsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal server error"))
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	_, err := c.PlaceOrder(context.Background(), "abc123", testOrderRequest())

	require.Error(t, err)

	var httpErr *apperr.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusInternalServerError, httpErr.StatusCode)
}

func TestPlaceOrder_NoLocationHeader(t *testing.T) {
	// 201 Created with no Location header should return empty PlaceOrderResponse.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	resp, err := c.PlaceOrder(context.Background(), "abc123", testOrderRequest())

	require.NoError(t, err)
	assert.Equal(t, int64(0), resp.OrderID)
}

func TestPlaceOrder_MalformedLocationHeader(t *testing.T) {
	// Location header with non-integer trailing segment should return a parse error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "/trader/v1/accounts/abc123/orders/not-a-number")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	_, err := c.PlaceOrder(context.Background(), "abc123", testOrderRequest())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse order ID from Location header")
}

func TestListOrders_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	_, err := c.ListOrders(context.Background(), "abc123", OrderListParams{})

	require.Error(t, err)

	var httpErr *apperr.HTTPError
	require.ErrorAs(t, err, &httpErr)
}

func TestAllOrders_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	_, err := c.AllOrders(context.Background(), OrderListParams{})

	require.Error(t, err)

	var httpErr *apperr.HTTPError
	require.ErrorAs(t, err, &httpErr)
}

func TestGetOrder_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	_, err := c.GetOrder(context.Background(), "abc123", 12345)

	require.Error(t, err)

	var httpErr *apperr.HTTPError
	require.ErrorAs(t, err, &httpErr)
}

func TestPreviewOrder_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	_, err := c.PreviewOrder(context.Background(), "abc123", testOrderRequest())

	require.Error(t, err)

	var httpErr *apperr.HTTPError
	require.ErrorAs(t, err, &httpErr)
}

func TestReplaceOrder_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	_, err := c.ReplaceOrder(context.Background(), "abc123", 12345, testOrderRequest())

	require.Error(t, err)

	var httpErr *apperr.HTTPError
	require.ErrorAs(t, err, &httpErr)
}

func TestCancelOrder_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient("test-token", WithBaseURL(srv.URL))
	err := c.CancelOrder(context.Background(), "abc123", 12345)

	require.Error(t, err)

	var httpErr *apperr.HTTPError
	require.ErrorAs(t, err, &httpErr)
}
