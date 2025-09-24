package droneweather

import (
	"testing"

	"agent-stack/internal/models"
	"agent-stack/shared/config"
)

// Test coordinate parsing utilities

func TestParseCoordinatePair(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedLat float64
		expectedLon float64
		wantErr     bool
	}{
		{
			name:        "Valid decimal coordinates",
			input:       "26.023333, -97.128333",
			expectedLat: 26.023333,
			expectedLon: -97.128333,
			wantErr:     false,
		},
		{
			name:        "Invalid format - single coordinate",
			input:       "26.023333",
			expectedLat: 0,
			expectedLon: 0,
			wantErr:     true,
		},
		{
			name:        "Invalid latitude",
			input:       "invalid, -97.128333",
			expectedLat: 0,
			expectedLon: 0,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lat, lon, err := parseCoordinatePair(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseCoordinatePair() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if abs(lat-tt.expectedLat) > 0.0001 || abs(lon-tt.expectedLon) > 0.0001 {
					t.Errorf("parseCoordinatePair() = %v, %v, want %v, %v", lat, lon, tt.expectedLat, tt.expectedLon)
				}
			}
		})
	}
}

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

func TestParseSimpleCoordinates(t *testing.T) {
	client := &TFRClient{}

	tests := []struct {
		name           string
		text           string
		expectFound    bool
		expectedLat    float64
		expectedLon    float64
		expectedRadius float64
	}{
		{
			name:           "Valid coordinates with radius",
			text:           "within 5 miles of 40.7128, -74.0060",
			expectFound:    true,
			expectedLat:    40.7128,
			expectedLon:    -74.0060,
			expectedRadius: 5.0,
		},
		{
			name:           "Coordinates without explicit radius",
			text:           "centered at 34.0522, -118.2437 for event",
			expectFound:    true,
			expectedLat:    34.0522,
			expectedLon:    -118.2437,
			expectedRadius: 10.0, // default
		},
		{
			name:           "No coordinates found",
			text:           "general flight restriction in the area",
			expectFound:    false,
			expectedLat:    0,
			expectedLon:    0,
			expectedRadius: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lat, lon, radius, found := client.parseSimpleCoordinates(tt.text)

			if found != tt.expectFound {
				t.Errorf("Expected found=%v, got %v", tt.expectFound, found)
			}

			if tt.expectFound {
				if abs(lat-tt.expectedLat) > 0.0001 {
					t.Errorf("Expected lat=%.4f, got %.4f", tt.expectedLat, lat)
				}
				if abs(lon-tt.expectedLon) > 0.0001 {
					t.Errorf("Expected lon=%.4f, got %.4f", tt.expectedLon, lon)
				}
				if abs(radius-tt.expectedRadius) > 0.1 {
					t.Errorf("Expected radius=%.1f, got %.1f", tt.expectedRadius, radius)
				}
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
