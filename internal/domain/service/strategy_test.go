package service_test

import (
	"errors"
	"testing"

	"activation-service/internal/domain/model"
	"activation-service/internal/domain/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAssetRepo is an in-test mock satisfying obtain.AssetRepository.
type mockAssetRepo struct {
	assets []model.Asset
	err    error
}

func (m *mockAssetRepo) FetchAllSortedByCost() ([]model.Asset, error) {
	return m.assets, m.err
}

func (m *mockAssetRepo) FetchPruned(date model.Date, volume int) ([]model.Asset, error) {
	if m.err != nil {
		return nil, m.err
	}
	// Filter by date and capacity <= volume * 1.5
	var result []model.Asset
	for _, a := range m.assets {
		if a.AvailDate.Equal(date) && float64(a.CapacityKW) <= float64(volume)*1.5 {
			result = append(result, a)
		}
	}
	return result, nil
}

// sampleAssets returns a fixed slice of 5 varying assets.
func sampleAssets() []model.Asset {
	return []model.Asset{
		{ID: 1, Name: "Alpha",   CapacityKW: 100, FixedCost: 500.0,  PricePerKW: 5.0,  AvailDate: model.MustParseDate("2026-06-01")},
		{ID: 2, Name: "Beta",    CapacityKW: 200, FixedCost: 800.0,  PricePerKW: 4.0,  AvailDate: model.MustParseDate("2026-06-01")},
		{ID: 3, Name: "Gamma",   CapacityKW: 150, FixedCost: 600.0,  PricePerKW: 4.0,  AvailDate: model.MustParseDate("2026-06-01")},
		{ID: 4, Name: "Delta",   CapacityKW: 300, FixedCost: 900.0,  PricePerKW: 3.0,  AvailDate: model.MustParseDate("2026-06-01")},
		{ID: 5, Name: "Epsilon", CapacityKW: 50,  FixedCost: 300.0,  PricePerKW: 6.0,  AvailDate: model.MustParseDate("2026-06-02")},
	}
}

// ---------------------------------------------------------------------------
// TestGreedyBaseline
// ---------------------------------------------------------------------------

func TestGreedyBaseline(t *testing.T) {
	t.Run("meets target volume with cheapest assets first", func(t *testing.T) {
		repo := &mockAssetRepo{assets: sampleAssets()}
		svc := service.NewGreedyBaseline(repo)

		req := model.ActivationRequest{Date: model.MustParseDate("2026-06-01"), TargetVolumeKW: 350}
		result, err := svc.Execute(req)
		assert.NoError(t, err)
		// Delta (300kW, $3/kW) and Beta/Gamma (tied at $4/kW) should be preferred over Alpha ($5/kW)
		assert.LessOrEqual(t, result.TotalCost, 500.0*float64(len(result.SelectedAssets))+1800.0)
	})

	t.Run("returns error when volume unattainable", func(t *testing.T) {
		repo := &mockAssetRepo{assets: sampleAssets()}
		svc := service.NewGreedyBaseline(repo)

		req := model.ActivationRequest{Date: model.MustParseDate("2026-06-01"), TargetVolumeKW: 10000}
		_, err := svc.Execute(req)

		assert.Error(t, err)
	})

	t.Run("repo error propagates", func(t *testing.T) {
		repo := &mockAssetRepo{err: errors.New("db error")}
		svc := service.NewGreedyBaseline(repo)

		_, err := svc.Execute(model.ActivationRequest{Date: model.MustParseDate("2026-06-01"), TargetVolumeKW: 100})
		assert.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// TestGreedyDB
// ---------------------------------------------------------------------------

func TestGreedyDB(t *testing.T) {
	t.Run("picks sequentially from pre-sorted list", func(t *testing.T) {
		// Simulate DB already sorted by price/kW asc, filtered by date
		preSorted := []model.Asset{
			{ID: 4, Name: "Delta",  CapacityKW: 300, FixedCost: 900.0, PricePerKW: 3.0, AvailDate: model.MustParseDate("2026-06-01")},
			{ID: 2, Name: "Beta",   CapacityKW: 200, FixedCost: 800.0, PricePerKW: 4.0, AvailDate: model.MustParseDate("2026-06-01")},
			{ID: 3, Name: "Gamma",  CapacityKW: 150, FixedCost: 600.0, PricePerKW: 4.0, AvailDate: model.MustParseDate("2026-06-01")},
			{ID: 1, Name: "Alpha",  CapacityKW: 100, FixedCost: 500.0, PricePerKW: 5.0, AvailDate: model.MustParseDate("2026-06-01")},
		}
		repo := &mockAssetRepo{assets: preSorted}
		svc := service.NewGreedyDB(repo)

		req := model.ActivationRequest{Date: model.MustParseDate("2026-06-01"), TargetVolumeKW: 300}
		result, err := svc.Execute(req)

		require.NoError(t, err)
		assert.GreaterOrEqual(t, result.TotalCapacityKW, 300)
		assert.Equal(t, 1, len(result.SelectedAssets))
		assert.Equal(t, "Delta", result.SelectedAssets[0].Name)
	})

	t.Run("returns error when volume unattainable", func(t *testing.T) {
		repo := &mockAssetRepo{assets: sampleAssets()}
		svc := service.NewGreedyDB(repo)

		req := model.ActivationRequest{Date: model.MustParseDate("2026-06-01"), TargetVolumeKW: 99999}
		_, err := svc.Execute(req)
		assert.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// TestKnapsackMemory
// ---------------------------------------------------------------------------

func TestKnapsackMemory(t *testing.T) {
	t.Run("minimizes fixed cost to exactly meet target", func(t *testing.T) {
		repo := &mockAssetRepo{assets: sampleAssets()}
		svc := service.NewKnapsackMemory(repo)

		req := model.ActivationRequest{Date: model.MustParseDate("2026-06-01"), TargetVolumeKW: 300}
		result, err := svc.Execute(req)

		require.NoError(t, err)
		assert.GreaterOrEqual(t, result.TotalCapacityKW, 300)
		// Knapsack should find Delta alone (300kW, $900) vs Beta+Gamma ($1400) — cost should be ≤ greedy
		assert.LessOrEqual(t, result.TotalCost, 1400.0)
	})

	t.Run("returns error when volume unattainable", func(t *testing.T) {
		repo := &mockAssetRepo{assets: sampleAssets()}
		svc := service.NewKnapsackMemory(repo)

		req := model.ActivationRequest{Date: model.MustParseDate("2026-06-01"), TargetVolumeKW: 99999}
		_, err := svc.Execute(req)
		assert.Error(t, err)
	})

	t.Run("repo error propagates", func(t *testing.T) {
		repo := &mockAssetRepo{err: errors.New("db error")}
		svc := service.NewKnapsackMemory(repo)

		_, err := svc.Execute(model.ActivationRequest{Date: model.MustParseDate("2026-06-01"), TargetVolumeKW: 100})
		assert.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// TestKnapsackDB
// ---------------------------------------------------------------------------

func TestKnapsackDB(t *testing.T) {
	t.Run("minimizes fixed cost on pre-pruned pool", func(t *testing.T) {
		// Simulate DB already pruned for date + capacity constraint
		pruned := []model.Asset{
			{ID: 4, Name: "Delta", CapacityKW: 300, FixedCost: 900.0,  PricePerKW: 3.0, AvailDate: model.MustParseDate("2026-06-01")},
			{ID: 2, Name: "Beta",  CapacityKW: 200, FixedCost: 800.0,  PricePerKW: 4.0, AvailDate: model.MustParseDate("2026-06-01")},
			{ID: 3, Name: "Gamma", CapacityKW: 150, FixedCost: 600.0,  PricePerKW: 4.0, AvailDate: model.MustParseDate("2026-06-01")},
		}
		repo := &mockAssetRepo{assets: pruned}
		svc := service.NewKnapsackDB(repo)

		req := model.ActivationRequest{Date: model.MustParseDate("2026-06-01"), TargetVolumeKW: 300}
		result, err := svc.Execute(req)

		require.NoError(t, err)
		assert.GreaterOrEqual(t, result.TotalCapacityKW, 300)
		// Delta alone meets the target at lower cost
		assert.LessOrEqual(t, result.TotalCost, 900.0)
	})

	t.Run("returns error when volume unattainable", func(t *testing.T) {
		repo := &mockAssetRepo{assets: []model.Asset{}}
		svc := service.NewKnapsackDB(repo)

		req := model.ActivationRequest{Date: model.MustParseDate("2026-06-01"), TargetVolumeKW: 500}
		_, err := svc.Execute(req)
		assert.Error(t, err)
	})
}
