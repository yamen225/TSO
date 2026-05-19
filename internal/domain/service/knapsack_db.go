package service

import (
	"activation-service/internal/domain/model"
	"activation-service/internal/domain/ports/obtain"
)

// KnapsackDB assumes the repository returns a pre-pruned pool (filtered by date
// and capacity constraint). It runs the exact same 0/1 DP algorithm.
type KnapsackDB struct {
	repo obtain.AssetRepository
}

func NewKnapsackDB(repo obtain.AssetRepository) *KnapsackDB {
	return &KnapsackDB{repo: repo}
}

func (k *KnapsackDB) Execute(req model.ActivationRequest) (model.AllocationResult, error) {
	assets, err := k.repo.FetchPruned(req.Date, req.TargetVolumeKW, req.EffectiveMultiplier())
	if err != nil {
		return model.AllocationResult{}, err
	}

	return knapsackSelect(assets, req.TargetVolumeKW)
}
