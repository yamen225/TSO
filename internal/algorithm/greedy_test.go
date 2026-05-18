package algorithm

import (
	"testing"

	"activation-service/internal/repository"
)

func TestOptimizeGreedy_Basic(t *testing.T) {
	assets := []repository.Asset{
		{ID: "A1", Name: "Asset 1", Volume: 100, ActivationCost: 150.0},
		{ID: "A2", Name: "Asset 2", Volume: 200, ActivationCost: 180.0},
		{ID: "A3", Name: "Asset 3", Volume: 50, ActivationCost: 60.0},
	}
	selected, totalCost := OptimizeGreedy(assets, 150)
	if len(selected) == 0 {
		t.Fatal("expected assets to be selected")
	}
	if totalCost <= 0 {
		t.Error("expected positive total cost")
	}
}

func TestOptimizeGreedy_ExactMatch(t *testing.T) {
	// A1: eff=1.0, A2: eff=1.6 — greedy picks A1 first (vol=100 meets target).
	assets := []repository.Asset{
		{ID: "A1", Name: "Asset 1", Volume: 100, ActivationCost: 100.0},
		{ID: "A2", Name: "Asset 2", Volume: 50, ActivationCost: 80.0},
	}
	selected, totalCost := OptimizeGreedy(assets, 100)
	if len(selected) != 1 {
		t.Fatalf("expected 1 asset selected, got %d", len(selected))
	}
	if totalCost != 100.0 {
		t.Errorf("expected cost 100.0, got %f", totalCost)
	}
}

func TestOptimizeGreedy_InsufficientCapacity(t *testing.T) {
	assets := []repository.Asset{
		{ID: "A1", Name: "Asset 1", Volume: 50, ActivationCost: 100.0},
	}
	selected, totalCost := OptimizeGreedy(assets, 200)
	if selected != nil {
		t.Error("expected nil when capacity is insufficient")
	}
	if totalCost != 0.0 {
		t.Errorf("expected 0 cost, got %f", totalCost)
	}
}

func TestOptimizeGreedy_EmptyAssets(t *testing.T) {
	selected, totalCost := OptimizeGreedy(nil, 100)
	if selected != nil {
		t.Error("expected nil for empty assets")
	}
	if totalCost != 0.0 {
		t.Errorf("expected 0 cost, got %f", totalCost)
	}
}

func TestOptimizeGreedy_ZeroTargetVolume(t *testing.T) {
	assets := []repository.Asset{
		{ID: "A1", Name: "Asset 1", Volume: 100, ActivationCost: 100.0},
	}
	selected, totalCost := OptimizeGreedy(assets, 0)
	if selected != nil {
		t.Error("expected nil for zero target volume")
	}
	if totalCost != 0.0 {
		t.Errorf("expected 0 cost, got %f", totalCost)
	}
}

func TestOptimizeGreedy_SelectsCheapestFirst(t *testing.T) {
	// A2: eff=0.5/kW (best), A3: eff=1.8/kW, A1: eff=2.0/kW
	// Target=100: greedy picks A2 alone (vol=200 >= 100).
	assets := []repository.Asset{
		{ID: "A1", Name: "Expensive", Volume: 100, ActivationCost: 200.0},
		{ID: "A2", Name: "Cheap", Volume: 200, ActivationCost: 100.0},
		{ID: "A3", Name: "Medium", Volume: 50, ActivationCost: 90.0},
	}
	selected, totalCost := OptimizeGreedy(assets, 100)
	if len(selected) != 1 {
		t.Fatalf("expected 1 asset selected, got %d", len(selected))
	}
	if selected[0].ID != "A2" {
		t.Errorf("expected cheapest asset A2 to be selected, got %s", selected[0].ID)
	}
	if totalCost != 100.0 {
		t.Errorf("expected cost 100.0, got %f", totalCost)
	}
}

func TestOptimizeGreedy_MultipleAssetsNeeded(t *testing.T) {
	// Target=250; single assets are 100 and 200 kW each.
	// A2 (eff=0.9) picked first, then A1 (eff=1.5) to cover remaining 150.
	assets := []repository.Asset{
		{ID: "A1", Name: "Asset 1", Volume: 200, ActivationCost: 300.0},
		{ID: "A2", Name: "Asset 2", Volume: 100, ActivationCost: 90.0},
	}
	selected, totalCost := OptimizeGreedy(assets, 250)
	if len(selected) != 2 {
		t.Fatalf("expected 2 assets, got %d", len(selected))
	}
	if totalCost != 390.0 {
		t.Errorf("expected cost 390.0, got %f", totalCost)
	}
}
