package algorithm

import (
	"sort"

	"activation-service/internal/repository"
)

// OptimizeGreedy selects assets in ascending cost-efficiency order until targetVolume is met.
// It sorts the assets slice in-place before iterating.
func OptimizeGreedy(assets []repository.Asset, targetVolume int) ([]repository.Asset, float64) {
	if targetVolume <= 0 || len(assets) == 0 {
		return nil, 0.0
	}

	// Sort by cost efficiency (Cost per kW) ascending.
	sort.Slice(assets, func(i, j int) bool {
		effI := assets[i].ActivationCost / float64(assets[i].Volume)
		effJ := assets[j].ActivationCost / float64(assets[j].Volume)
		return effI < effJ
	})

	var selected []repository.Asset
	var totalVolume int
	var totalCost float64

	for _, asset := range assets {
		if totalVolume >= targetVolume {
			break
		}
		selected = append(selected, asset)
		totalVolume += asset.Volume
		totalCost += asset.ActivationCost
	}

	if totalVolume < targetVolume {
		return nil, 0.0 // target volume unattainable
	}

	return selected, totalCost
}
