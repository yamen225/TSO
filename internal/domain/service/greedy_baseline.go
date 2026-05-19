package service

import (
	"activation-service/internal/domain/model"
	"activation-service/internal/domain/ports/obtain"
)

// GreedyBaseline delegates date filtering to the repository, then picks
// sequentially until the target volume is reached.
type GreedyBaseline struct {
	repo obtain.AssetRepository
}

func NewGreedyBaseline(repo obtain.AssetRepository) *GreedyBaseline {
	return &GreedyBaseline{repo: repo}
}

func (g *GreedyBaseline) Execute(req model.ActivationRequest) (model.AllocationResult, error) {
	available, err := g.repo.FetchAllSortedByCost(req.Date)
	if err != nil {
		return model.AllocationResult{}, err
	}

	return greedySelect(available, req.TargetVolumeKW)
}
