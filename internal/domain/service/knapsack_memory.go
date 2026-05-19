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
	all, err := k.repo.FetchAllSortedByCost()
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

	return knapsackSelect(available, req.TargetVolumeKW)
}
