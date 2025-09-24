package droneweather

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"time"

	"agent-stack/internal/models"
	"agent-stack/shared/config"
)

// TFRClient handles interactions with the FAA TFR API
type TFRClient struct {
	config *config.DroneWeatherConfig
	client *http.Client
}

func NewTFRClient(cfg *config.DroneWeatherConfig) *TFRClient {
	return &TFRClient{
		config: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CheckTFRs checks for active TFRs in the area around the given coordinates
func (t *TFRClient) CheckTFRs(ctx context.Context, lat, lon float64) (*models.TFRCheck, error) {
	log.Printf("Checking TFRs around %.4f, %.4f within %d miles", lat, lon, t.config.SearchRadiusMiles)

	// Calculate bounding box for the search area
	radiusKm := float64(t.config.SearchRadiusMiles) * 1.60934
	latDelta := radiusKm / 111.0 // Rough km per degree latitude
	lonDelta := radiusKm / (111.0 * math.Cos(lat*math.Pi/180))

	minLat := lat - latDelta
	maxLat := lat + latDelta
	minLon := lon - lonDelta
	maxLon := lon + lonDelta

	log.Printf("Search box: %.4f,%.4f to %.4f,%.4f", minLat, minLon, maxLat, maxLon)

	// For simplicity, we'll use a mock check since the real FAA TFR API
	// requires complex HTML/XML parsing and may have rate limits.
	// In a production system, this would:
	// 1. Query the FAA TFR service with the bounding box
	// 2. Parse the returned TFR data (usually HTML or XML)
	// 3. Filter for active TFRs within the time window
	// 4. Check if any TFRs intersect with our search area

	activeTFRs := t.mockTFRCheck(ctx, lat, lon)

	check := &models.TFRCheck{
		HasActiveTFRs: len(activeTFRs) > 0,
		ActiveTFRs:    activeTFRs,
		CheckRadius:   t.config.SearchRadiusMiles,
		CheckTime:     time.Now(),
	}

	if len(activeTFRs) == 0 {
		check.Summary = fmt.Sprintf("No restrictions found within %d miles - clear to fly", t.config.SearchRadiusMiles)
	} else {
		check.Summary = fmt.Sprintf("%d restriction(s) found within %d miles - check locations before flying", len(activeTFRs), t.config.SearchRadiusMiles)
	}

	return check, nil
}

// mockTFRCheck provides a mock TFR check for demonstration
// In production, this would be replaced with real FAA API calls
func (t *TFRClient) mockTFRCheck(ctx context.Context, lat, lon float64) []*models.TFR {
	// For demonstration purposes, we'll return no active TFRs
	// In a real implementation, this would:
	// 1. Make HTTP requests to FAA TFR services
	// 2. Parse the response (often HTML or XML)
	// 3. Extract TFR information including coordinates, times, and restrictions
	// 4. Return only TFRs that are currently active and within range

	log.Printf("Mock TFR check: No active TFRs found (this would query real FAA API in production)")

	// Example of what an active TFR might look like:
	// return []*models.TFR{
	// 	{
	// 		ID:        "TFR-2024-001",
	// 		Name:      "Presidential TFR",
	// 		Type:      "Security",
	// 		StartTime: time.Now().Add(-1 * time.Hour),
	// 		EndTime:   time.Now().Add(2 * time.Hour),
	// 		Latitude:  lat + 0.1,
	// 		Longitude: lon + 0.1,
	// 		Radius:    10, // nautical miles
	// 		AltMin:    0,
	// 		AltMax:    18000,
	// 		Reason:    "Presidential movement",
	// 	},
	// }

	return []*models.TFR{} // No active TFRs for demo
}

// isWithinRadius checks if a TFR is within the specified radius of the home location
func (t *TFRClient) isWithinRadius(homeLat, homeLon, tfrLat, tfrLon float64, radiusMiles int) bool {
	distance := t.calculateDistance(homeLat, homeLon, tfrLat, tfrLon)
	return distance <= float64(radiusMiles)
}

// calculateDistance calculates the distance between two coordinates in miles
func (t *TFRClient) calculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusMiles = 3959.0

	lat1Rad := lat1 * math.Pi / 180
	lon1Rad := lon1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	lon2Rad := lon2 * math.Pi / 180

	dlat := lat2Rad - lat1Rad
	dlon := lon2Rad - lon1Rad

	a := math.Sin(dlat/2)*math.Sin(dlat/2) + math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(dlon/2)*math.Sin(dlon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusMiles * c
}