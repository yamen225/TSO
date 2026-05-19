package service

import (
	"sort"

	"activation-service/internal/domain/model"
	"activation-service/internal/domain/ports/obtain"
)

// GreedyBaseline filters assets by date in memory, sorts by price/kW ascending,
// then picks sequentially until the target volume is reached.
type GreedyBaseline struct {
	repo obtain.AssetRepository
}

func NewGreedyBaseline(repo obtain.AssetRepository) *GreedyBaseline {
	return &GreedyBaseline{repo: repo}
}

func (g *GreedyBaseline) Execute(req model.ActivationRequest) (model.AllocationResult, error) {
	all, err := g.repo.FetchAllSortedByCost()
	if err != nil {
		return model.AllocationResult{}, err
	}

	// Filter by date
	var available []model.Asset
	for _, a := range all {
		if a.AvailDate.Equal(req.Date) {
			available = append(available, a)
		}
	}

	// Sort by price/kW ascending
	sort.Slice(available, func(i, j int) bool {
		return available[i].PricePerKW < available[j].PricePerKW
	})

	return greedySelect(available, req.TargetVolumeKW)
}
