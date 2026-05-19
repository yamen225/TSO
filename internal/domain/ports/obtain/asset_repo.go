package obtain

import "activation-service/internal/domain/model"

// AssetRepository is the outbound port for fetching assets.
type AssetRepository interface {
	// FetchAllSortedByCost returns assets available on date ordered by cost_per_kw ascending.
	// Implementors MUST guarantee this ordering; callers such as GreedyBaseline
	// and KnapsackMemory depend on it for correctness.
	FetchAllSortedByCost(date model.Date) ([]model.Asset, error)
	// FetchPruned returns assets available on date with capacity <= target * multiplier,
	// ordered by cost_per_kw ascending.
	FetchPruned(date model.Date, volume int, multiplier float64) ([]model.Asset, error)
}
