package obtain

import "activation-service/internal/domain/model"

// AssetRepository is the outbound port for fetching assets.
type AssetRepository interface {
	// FetchAll returns all assets ordered by cost_per_kw ascending.
	FetchAll() ([]model.Asset, error)
	// FetchPruned returns assets available on date with capacity <= target * 1.5,
	// ordered by cost_per_kw ascending.
	FetchPruned(date string, volume int) ([]model.Asset, error)
}
