package repository

import (
	"testing"
)

func TestGetAssetsByDate_ReturnsOnlyMatchingDate(t *testing.T) {
	assets := GetAssetsByDate("2024-01-15")
	if len(assets) == 0 {
		t.Fatal("expected assets for 2024-01-15")
	}
	for _, a := range assets {
		found := false
		for _, d := range a.Availability {
			if d == "2024-01-15" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("asset %s is not available on 2024-01-15", a.ID)
		}
	}
}

func TestGetAssetsByDate_NoMatch(t *testing.T) {
	assets := GetAssetsByDate("1999-01-01")
	if len(assets) != 0 {
		t.Errorf("expected 0 assets for non-existent date, got %d", len(assets))
	}
}

func TestGetMockAssets_NotEmpty(t *testing.T) {
	assets := GetMockAssets()
	if len(assets) == 0 {
		t.Fatal("mock assets should not be empty")
	}
	for _, a := range assets {
		if a.ID == "" {
			t.Error("asset ID must not be empty")
		}
		if a.Volume <= 0 {
			t.Errorf("asset %s volume must be positive, got %d", a.ID, a.Volume)
		}
		if a.ActivationCost <= 0 {
			t.Errorf("asset %s activation cost must be positive, got %f", a.ID, a.ActivationCost)
		}
	}
}
