package service

import (
	"activation-service/internal/domain/model"
	"activation-service/internal/domain/ports/obtain"
)

// KnapsackMemory filters by date in memory then runs a 0/1 bottom-up dynamic
// programming algorithm to find the minimum fixed cost that meets target volume.
type KnapsackMemory struct {
	repo obtain.AssetRepository
}

func NewKnapsackMemory(repo obtain.AssetRepository) *KnapsackMemory {
	return &KnapsackMemory{repo: repo}
}

func (k *KnapsackMemory) Execute(req model.ActivationRequest) (model.AllocationResult, error) {
	available, err := k.repo.FetchAllSortedByCost(req.Date)
	if err != nil {
		return model.AllocationResult{}, err
	}

	return knapsackSelect(available, req.TargetVolumeKW)
}
