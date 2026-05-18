package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOptimizeHandler_ValidRequest(t *testing.T) {
	body, _ := json.Marshal(OptimizeRequest{Date: "2024-01-15", TargetVolume: 100})
	req := httptest.NewRequest(http.MethodPost, "/optimize", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	OptimizeHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp OptimizeResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal("failed to decode response:", err)
	}
	if len(resp.SelectedAssets) == 0 {
		t.Error("expected at least one selected asset")
	}
	if resp.TotalCost <= 0 {
		t.Error("expected positive total cost")
	}
	if resp.Algorithm != "greedy" {
		t.Errorf("expected algorithm 'greedy', got '%s'", resp.Algorithm)
	}
}

func TestOptimizeHandler_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/optimize", nil)
	rr := httptest.NewRecorder()

	OptimizeHandler(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestOptimizeHandler_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/optimize", bytes.NewReader([]byte("not-json")))
	rr := httptest.NewRecorder()

	OptimizeHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestOptimizeHandler_MissingDate(t *testing.T) {
	body, _ := json.Marshal(OptimizeRequest{TargetVolume: 100})
	req := httptest.NewRequest(http.MethodPost, "/optimize", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	OptimizeHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestOptimizeHandler_ZeroTargetVolume(t *testing.T) {
	body, _ := json.Marshal(OptimizeRequest{Date: "2024-01-15", TargetVolume: 0})
	req := httptest.NewRequest(http.MethodPost, "/optimize", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	OptimizeHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestOptimizeHandler_UnattainableVolume(t *testing.T) {
	// Date with very few assets, target volume impossible to reach.
	body, _ := json.Marshal(OptimizeRequest{Date: "2024-01-15", TargetVolume: 99999})
	req := httptest.NewRequest(http.MethodPost, "/optimize", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	OptimizeHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp OptimizeResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if len(resp.SelectedAssets) != 0 {
		t.Error("expected no assets when volume is unattainable")
	}
}
