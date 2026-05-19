package handler_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"activation-service/internal/app/handler"
	"activation-service/internal/domain/model"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// mockUseCase satisfies request.ActivationUseCase for testing.
type mockUseCase struct {
	result model.AllocationResult
	err    error
}

func (m *mockUseCase) Execute(req model.ActivationRequest) (model.AllocationResult, error) {
	return m.result, m.err
}

var sampleResult = model.AllocationResult{
	SelectedAssets:  []model.Asset{{ID: 1, Name: "Delta", CapacityKW: 300, FixedCost: 900.0}},
	TotalCapacityKW: 300,
	TotalCost:       900.0,
}

var validPayload = model.ActivationRequest{Date: model.MustParseDate("2026-06-01"), TargetVolumeKW: 300}

func setupRouter(h *handler.ActivationGinHandler) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1/activation")
	v1.POST("/greedy-baseline", h.HandleGreedyBaseline)
	v1.POST("/greedy-db", h.HandleGreedyDB)
	v1.POST("/knapsack-memory", h.HandleKnapsackMemory)
	v1.POST("/knapsack-db", h.HandleKnapsackDB)
	return r
}

func doPost(t *testing.T, router *gin.Engine, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// ---------------------------------------------------------------------------
// 200 OK tests for all 4 endpoints
// ---------------------------------------------------------------------------

func TestHandleGreedyBaseline_OK(t *testing.T) {
	uc := &mockUseCase{result: sampleResult}
	h := handler.NewActivationGinHandler(uc, uc, uc, uc)
	w := doPost(t, setupRouter(h), "/api/v1/activation/greedy-baseline", validPayload)

	assert.Equal(t, http.StatusOK, w.Code)
	var res model.AllocationResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	assert.Equal(t, 300, res.TotalCapacityKW)
}

func TestHandleGreedyDB_OK(t *testing.T) {
	uc := &mockUseCase{result: sampleResult}
	h := handler.NewActivationGinHandler(uc, uc, uc, uc)
	w := doPost(t, setupRouter(h), "/api/v1/activation/greedy-db", validPayload)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleKnapsackMemory_OK(t *testing.T) {
	uc := &mockUseCase{result: sampleResult}
	h := handler.NewActivationGinHandler(uc, uc, uc, uc)
	w := doPost(t, setupRouter(h), "/api/v1/activation/knapsack-memory", validPayload)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleKnapsackDB_OK(t *testing.T) {
	uc := &mockUseCase{result: sampleResult}
	h := handler.NewActivationGinHandler(uc, uc, uc, uc)
	w := doPost(t, setupRouter(h), "/api/v1/activation/knapsack-db", validPayload)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ---------------------------------------------------------------------------
// 400 Bad Request on invalid JSON
// ---------------------------------------------------------------------------

func TestHandleGreedyBaseline_BadRequest(t *testing.T) {
	uc := &mockUseCase{}
	h := handler.NewActivationGinHandler(uc, uc, uc, uc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/activation/greedy-baseline",
		bytes.NewBufferString("{invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// 500 when use case returns error
// ---------------------------------------------------------------------------

func TestHandleGreedyBaseline_ServiceError(t *testing.T) {
	uc := &mockUseCase{err: errors.New("volume unattainable")}
	h := handler.NewActivationGinHandler(uc, uc, uc, uc)
	w := doPost(t, setupRouter(h), "/api/v1/activation/greedy-baseline", validPayload)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}
