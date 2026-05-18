package handler

import (
	"encoding/json"
	"net/http"

	"activation-service/internal/algorithm"
	"activation-service/internal/repository"
)

// OptimizeRequest is the JSON body for the optimize endpoint.
type OptimizeRequest struct {
	Date         string `json:"date"`
	TargetVolume int    `json:"target_volume"`
}

// OptimizeResponse is the JSON response for the optimize endpoint.
type OptimizeResponse struct {
	SelectedAssets []repository.Asset `json:"selected_assets"`
	TotalCost      float64            `json:"total_cost"`
	Algorithm      string             `json:"algorithm"`
}

// OptimizeHandler handles POST /optimize requests using the greedy algorithm.
func OptimizeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req OptimizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Date == "" || req.TargetVolume <= 0 {
		http.Error(w, "date and positive target_volume are required", http.StatusBadRequest)
		return
	}

	assets := repository.GetAssetsByDate(req.Date)
	selected, totalCost := algorithm.OptimizeGreedy(assets, req.TargetVolume)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(OptimizeResponse{
		SelectedAssets: selected,
		TotalCost:      totalCost,
		Algorithm:      "greedy",
	})
}
