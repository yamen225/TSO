package repository

// Asset represents a dispatchable energy asset.
type Asset struct {
	ID             string
	Name           string
	Volume         int      // capacity in kW
	ActivationCost float64  // fixed cost to activate
	Availability   []string // available dates in YYYY-MM-DD format
}

// GetMockAssets returns the static mock dataset of assets.
func GetMockAssets() []Asset {
	return []Asset{
		{ID: "A1", Name: "Solar Farm Alpha", Volume: 100, ActivationCost: 150.0, Availability: []string{"2024-01-15", "2024-01-16", "2024-01-17"}},
		{ID: "A2", Name: "Wind Turbine Beta", Volume: 200, ActivationCost: 180.0, Availability: []string{"2024-01-15", "2024-01-18"}},
		{ID: "A3", Name: "Battery Storage Gamma", Volume: 50, ActivationCost: 60.0, Availability: []string{"2024-01-15", "2024-01-16"}},
		{ID: "A4", Name: "Hydro Plant Delta", Volume: 300, ActivationCost: 400.0, Availability: []string{"2024-01-15"}},
		{ID: "A5", Name: "Gas Peaker Epsilon", Volume: 150, ActivationCost: 220.0, Availability: []string{"2024-01-16", "2024-01-17"}},
		{ID: "A6", Name: "Diesel Gen Zeta", Volume: 75, ActivationCost: 90.0, Availability: []string{"2024-01-15", "2024-01-16", "2024-01-17", "2024-01-18"}},
		{ID: "A7", Name: "Biomass Eta", Volume: 120, ActivationCost: 160.0, Availability: []string{"2024-01-15", "2024-01-17"}},
		{ID: "A8", Name: "Tidal Theta", Volume: 80, ActivationCost: 95.0, Availability: []string{"2024-01-16", "2024-01-18"}},
	}
}

// GetAssetsByDate filters assets available on the given date.
func GetAssetsByDate(date string) []Asset {
	allAssets := GetMockAssets()
	var filtered []Asset
	for _, asset := range allAssets {
		for _, avDate := range asset.Availability {
			if avDate == date {
				filtered = append(filtered, asset)
				break
			}
		}
	}
	return filtered
}
