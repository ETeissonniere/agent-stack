package droneweather

import (
	"testing"

	"agent-stack/internal/models"
	"agent-stack/shared/config"
)

// Test distance calculations

func TestCalculateDistance(t *testing.T) {
	client := &TFRClient{}

	tests := []struct {
		name      string
		lat1      float64
		lon1      float64
		lat2      float64
		lon2      float64
		expected  float64 // approximate distance in miles
		tolerance float64
	}{
		{
			name: "Same point",
			lat1: 40.7128, lon1: -74.0060,
			lat2: 40.7128, lon2: -74.0060,
			expected:  0,
			tolerance: 0.1,
		},
		{
			name: "NYC to LA (approximately)",
			lat1: 40.7128, lon1: -74.0060, // NYC
			lat2: 34.0522, lon2: -118.2437, // LA
			expected:  2445, // ~2445 miles
			tolerance: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.calculateDistance(tt.lat1, tt.lon1, tt.lat2, tt.lon2)
			if abs(result-tt.expected) > tt.tolerance {
				t.Errorf("calculateDistance() = %v, want %v ± %v", result, tt.expected, tt.tolerance)
			}
		})
	}
}

func TestIsWithinSearchArea(t *testing.T) {
	client := &TFRClient{config: &config.DroneWeatherConfig{SearchRadiusMiles: 25}}

	tests := []struct {
		name     string
		homeLat  float64
		homeLon  float64
		tfr      *models.TFR
		expected bool
	}{
		{
			name:    "TFR within search area",
			homeLat: 40.0, homeLon: -74.0,
			tfr:      &models.TFR{Latitude: 40.1, Longitude: -74.1, Radius: 10},
			expected: true,
		},
		{
			name:    "TFR outside search area",
			homeLat: 40.0, homeLon: -74.0,
			tfr:      &models.TFR{Latitude: 42.0, Longitude: -76.0, Radius: 5},
			expected: false,
		},
		{
			name:    "TFR with no coordinates",
			homeLat: 40.0, homeLon: -74.0,
			tfr:      &models.TFR{Latitude: 0, Longitude: 0, Radius: 10},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.isWithinSearchArea(tt.homeLat, tt.homeLon, tt.tfr)
			if result != tt.expected {
				t.Errorf("isWithinSearchArea() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestBuildTFRCheck(t *testing.T) {
	client := &TFRClient{config: &config.DroneWeatherConfig{SearchRadiusMiles: 25}}

	tests := []struct {
		name          string
		activeTFRs    []*models.TFR
		expectActive  bool
		expectSummary string
	}{
		{
			name:          "No active TFRs",
			activeTFRs:    []*models.TFR{},
			expectActive:  false,
			expectSummary: "No restrictions found within 25 miles - clear to fly",
		},
		{
			name: "One active TFR",
			activeTFRs: []*models.TFR{
				{ID: "TFR001", Type: "SPORTS", Reason: "Baseball game"},
			},
			expectActive:  true,
			expectSummary: "1 restriction(s) found within 25 miles - check locations before flying",
		},
		{
			name: "Multiple active TFRs",
			activeTFRs: []*models.TFR{
				{ID: "TFR001", Type: "SPORTS", Reason: "Baseball game"},
				{ID: "TFR002", Type: "HAZARDS", Reason: "Wildfire"},
			},
			expectActive:  true,
			expectSummary: "2 restriction(s) found within 25 miles - check locations before flying",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := client.buildTFRCheck(tt.activeTFRs)

			if check.HasActiveTFRs != tt.expectActive {
				t.Errorf("Expected HasActiveTFRs=%v, got %v", tt.expectActive, check.HasActiveTFRs)
			}

			if check.Summary != tt.expectSummary {
				t.Errorf("Expected summary '%s', got '%s'", tt.expectSummary, check.Summary)
			}

			if len(check.ActiveTFRs) != len(tt.activeTFRs) {
				t.Errorf("Expected %d TFRs, got %d", len(tt.activeTFRs), len(check.ActiveTFRs))
			}

			if check.CheckRadius != 25 {
				t.Errorf("Expected CheckRadius=25, got %d", check.CheckRadius)
			}
		})
	}
}

// Helper function for floating point comparison
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
