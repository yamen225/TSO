package model

// Asset represents a TSO electrical asset that can be activated.
type Asset struct {
	ID          int
	Name        string
	CapacityKW  int
	FixedCost   float64
	PricePerKW  float64
	AvailDate   Date
}

// ActivationRequest is the inbound request to the domain use case.
type ActivationRequest struct {
	Date           Date    `json:"date"`
	TargetVolumeKW int     `json:"target_volume_kw"`
	CapMultiplier  float64 `json:"cap_multiplier,omitempty"`
}

// EffectiveMultiplier returns CapMultiplier if set, otherwise the default of 1.5.
func (r ActivationRequest) EffectiveMultiplier() float64 {
	if r.CapMultiplier <= 0 {
		return 1.5
	}
	return r.CapMultiplier
}

// AllocationResult holds the result of an asset activation decision.
type AllocationResult struct {
	SelectedAssets []Asset `json:"selected_assets"`
	TotalCapacityKW int    `json:"total_capacity_kw"`
	TotalCost       float64 `json:"total_cost"`
}
