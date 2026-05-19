package repo

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"activation-service/internal/domain/model"
)

// assetRow is the GORM scan target for the joined query result.
type assetRow struct {
	ID         int     `gorm:"column:id"`
	Name       string  `gorm:"column:name"`
	CapacityKW int     `gorm:"column:capacity_kw"`
	FixedCost  float64 `gorm:"column:fixed_cost"`
	PricePerKW float64 `gorm:"column:price_per_kw"`
	AvailDate  model.Date `gorm:"column:avail_date"`
}

// PostgresAssetRepository is the outbound adapter for asset data, backed by GORM.
type PostgresAssetRepository struct {
	db *gorm.DB
}

// NewPostgresAssetRepository creates a repository from an existing *gorm.DB (used in tests).
func NewPostgresAssetRepository(db *gorm.DB) *PostgresAssetRepository {
	return &PostgresAssetRepository{db: db}
}

// NewPostgresAssetRepositoryFromURL opens a GORM connection from DATABASE_URL.
func NewPostgresAssetRepositoryFromURL(connStr string) (*PostgresAssetRepository, error) {
	db, err := gorm.Open(postgres.Open(connStr), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("opening gorm db: %w", err)
	}
	return &PostgresAssetRepository{db: db}, nil
}

const (
	selectColumns = "assets.id, assets.name, assets.capacity_kw, assets.fixed_cost, assets.price_per_kw, " +
		"asset_availabilities.avail_date"
	joinClause = "JOIN asset_availabilities ON asset_availabilities.asset_id = assets.id"
)

// FetchAllSortedByCost returns all assets ordered by price_per_kw ascending.
func (r *PostgresAssetRepository) FetchAllSortedByCost() ([]model.Asset, error) {
	var rows []assetRow
	result := r.db.Table("assets").
		Select(selectColumns).
		Joins(joinClause).
		Order("assets.price_per_kw ASC").
		Scan(&rows)
	if result.Error != nil {
		return nil, fmt.Errorf("FetchAllSortedByCost: %w", result.Error)
	}
	return mapRows(rows), nil
}

// FetchPruned returns assets available on date with capacity <= volume * 1.5.
func (r *PostgresAssetRepository) FetchPruned(date model.Date, volume int) ([]model.Asset, error) {
	maxCap := float64(volume) * 1.5
	var rows []assetRow
	result := r.db.Table("assets").
		Select(selectColumns).
		Joins(joinClause).
		Where("asset_availabilities.avail_date = ? AND assets.capacity_kw <= ?", date, maxCap).
		Order("assets.price_per_kw ASC").
		Scan(&rows)
	if result.Error != nil {
		return nil, fmt.Errorf("FetchPruned: %w", result.Error)
	}
	return mapRows(rows), nil
}

func mapRows(rows []assetRow) []model.Asset {
	assets := make([]model.Asset, len(rows))
	for i, r := range rows {
		assets[i] = model.Asset{
			ID:         r.ID,
			Name:       r.Name,
			CapacityKW: r.CapacityKW,
			FixedCost:  r.FixedCost,
			PricePerKW: r.PricePerKW,
			AvailDate:  r.AvailDate,
		}
	}
	return assets
}
