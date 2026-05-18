package service

import (
	"errors"
	"math"

	"activation-service/internal/domain/model"
)

var ErrVolumeUnattainable = errors.New("target volume cannot be met with available assets")

// greedySelect picks assets sequentially from the (already sorted/filtered)
// slice until targetKW is satisfied. Returns error if total capacity falls short.
func greedySelect(assets []model.Asset, targetKW int) (model.AllocationResult, error) {
	var selected []model.Asset
	totalCap := 0
	totalCost := 0.0

	for _, a := range assets {
		if totalCap >= targetKW {
			break
		}
		selected = append(selected, a)
		totalCap += a.CapacityKW
		totalCost += a.FixedCost
	}

	if totalCap < targetKW {
		return model.AllocationResult{}, ErrVolumeUnattainable
	}

	return model.AllocationResult{
		SelectedAssets:  selected,
		TotalCapacityKW: totalCap,
		TotalCost:       totalCost,
	}, nil
}

// knapsackSelect runs a 0/1 bottom-up DP to find the combination of assets
// with the minimum total FixedCost that meets or exceeds targetKW.
//
// The DP table: dp[cap] = minimum cost to achieve exactly `cap` kW.
// We treat the problem as: find minimum cost subset with total capacity >= targetKW.
func knapsackSelect(assets []model.Asset, targetKW int) (model.AllocationResult, error) {
	if len(assets) == 0 {
		return model.AllocationResult{}, ErrVolumeUnattainable
	}

	// Calculate the maximum possible capacity.
	maxCap := 0
	for _, a := range assets {
		maxCap += a.CapacityKW
	}
	if maxCap < targetKW {
		return model.AllocationResult{}, ErrVolumeUnattainable
	}

	// dp[c] = minimum fixed cost to reach exactly capacity c.
	// Use math.MaxFloat64 as "infinity" (unreachable state).
	dp := make([]float64, maxCap+1)
	for i := range dp {
		dp[i] = math.MaxFloat64
	}
	dp[0] = 0.0

	// chosen[c] = index of last asset added to reach capacity c.
	chosen := make([]int, maxCap+1)
	for i := range chosen {
		chosen[i] = -1
	}

	// Standard 0/1 knapsack — iterate backwards to avoid reuse.
	for idx, a := range assets {
		for c := maxCap; c >= a.CapacityKW; c-- {
			prev := c - a.CapacityKW
			if dp[prev] == math.MaxFloat64 {
				continue
			}
			candidate := dp[prev] + a.FixedCost
			if candidate < dp[c] {
				dp[c] = candidate
				chosen[c] = idx
			}
		}
	}

	// Find the minimum cost capacity >= targetKW.
	bestCap := -1
	bestCost := math.MaxFloat64
	for c := targetKW; c <= maxCap; c++ {
		if dp[c] < bestCost {
			bestCost = dp[c]
			bestCap = c
		}
	}

	if bestCap == -1 {
		return model.AllocationResult{}, ErrVolumeUnattainable
	}

	// Backtrack to recover selected assets.
	used := make([]bool, len(assets))
	cap := bestCap
	for cap > 0 && chosen[cap] != -1 {
		idx := chosen[cap]
		if used[idx] {
			break
		}
		used[idx] = true
		cap -= assets[idx].CapacityKW
	}

	var selected []model.Asset
	totalCap := 0
	totalCost := 0.0
	for i, a := range assets {
		if used[i] {
			selected = append(selected, a)
			totalCap += a.CapacityKW
			totalCost += a.FixedCost
		}
	}

	return model.AllocationResult{
		SelectedAssets:  selected,
		TotalCapacityKW: totalCap,
		TotalCost:       totalCost,
	}, nil
}
