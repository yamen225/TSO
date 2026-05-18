package service

import (
	"activation-service/internal/domain/model"
	"activation-service/internal/domain/ports/obtain"
)

// GreedyDB assumes the repository returns assets already filtered by date
// and sorted by price/kW ascending. It picks sequentially until target is met.
type GreedyDB struct {
	repo obtain.AssetRepository
}

func NewGreedyDB(repo obtain.AssetRepository) *GreedyDB {
	return &GreedyDB{repo: repo}
}

func (g *GreedyDB) Execute(req model.ActivationRequest) (model.AllocationResult, error) {
	assets, err := g.repo.FetchPruned(req.Date, req.TargetVolumeKW)
	if err != nil {
		return model.AllocationResult{}, err
	}

	return greedySelect(assets, req.TargetVolumeKW)
}
