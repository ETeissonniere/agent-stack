package droneweather

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"regexp"
	"strings"
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

// GeoJSON structures for parsing TFR data
type GeoJSONFeatureCollection struct {
	Type     string           `json:"type"`
	Features []GeoJSONFeature `json:"features"`
}

type GeoJSONFeature struct {
	Type       string                 `json:"type"`
	Properties GeoJSONProperties      `json:"properties"`
	Geometry   GeoJSONGeometry        `json:"geometry"`
}

type GeoJSONProperties struct {
	NotamKey   string `json:"NOTAM_KEY"`
	LegalClass string `json:"LEGAL"`
	Title      string `json:"TITLE"`
	State      string `json:"STATE"`
}

type GeoJSONGeometry struct {
	Type        string          `json:"type"`
	Coordinates [][][]float64   `json:"coordinates"`
}

// TFR fetching and parsing functions

// fetchActiveTFRs fetches the list of active TFRs from FAA GeoJSON API
func (t *TFRClient) fetchActiveTFRs(ctx context.Context) ([]*models.TFR, error) {
	log.Printf("Fetching fresh TFR data")

	// Use the FAA GeoServer WFS endpoint for TFR data
	endpoint := "https://tfr.faa.gov/geoserver/TFR/ows?service=WFS&version=1.1.0&request=GetFeature&typeName=TFR:V_TFR_LOC&maxFeatures=300&outputFormat=application/json&srsname=EPSG:3857"
	log.Printf("Fetching TFRs from: %s", endpoint)

	tfrs, err := t.fetchFromEndpoint(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch TFRs from %s: %w", endpoint, err)
	}

	log.Printf("Successfully fetched %d TFRs", len(tfrs))
	return tfrs, nil
}

// fetchFromEndpoint attempts to fetch TFR data from a specific endpoint
func (t *TFRClient) fetchFromEndpoint(ctx context.Context, endpoint string) ([]*models.TFR, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Set headers to mimic browser request
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; DroneWeatherBot/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Parse GeoJSON response
	return t.parseGeoJSONTFRs(resp.Body)
}

// parseGeoJSONTFRs parses TFR data from GeoJSON content
func (t *TFRClient) parseGeoJSONTFRs(body io.Reader) ([]*models.TFR, error) {
	var featureCollection GeoJSONFeatureCollection
	if err := json.NewDecoder(body).Decode(&featureCollection); err != nil {
		return nil, fmt.Errorf("parsing GeoJSON: %w", err)
	}

	var tfrs []*models.TFR

	for _, feature := range featureCollection.Features {
		tfr := &models.TFR{}

		// Extract basic properties
		tfr.ID = feature.Properties.NotamKey
		tfr.Type = feature.Properties.LegalClass
		tfr.Name = feature.Properties.State

		// Parse dates from title
		startTime, endTime, err := t.parseTFRDatesFromTitle(feature.Properties.Title)
		if err != nil {
			// For TFRs without clear date patterns (permanent restrictions), assume they're active
			log.Printf("Using default dates for TFR %s (likely permanent): %v", tfr.ID, err)
			tfr.StartTime = time.Now().Add(-24 * time.Hour) // Started yesterday
			tfr.EndTime = time.Now().Add(365 * 24 * time.Hour) // Valid for a year
		} else {
			tfr.StartTime = startTime
			tfr.EndTime = endTime
		}

		// Calculate center point and radius from polygon
		if feature.Geometry.Type == "Polygon" && len(feature.Geometry.Coordinates) > 0 {
			lat, lon, radius := t.calculatePolygonCenter(feature.Geometry.Coordinates[0])
			tfr.Latitude = lat
			tfr.Longitude = lon
			tfr.Radius = radius
		}

		// Only add if we have basic info
		if tfr.ID != "" && tfr.Type != "" {
			tfrs = append(tfrs, tfr)
		}
	}

	return tfrs, nil
}

// parseTFRDatesFromTitle parses dates from TFR title format
func (t *TFRClient) parseTFRDatesFromTitle(title string) (startTime, endTime time.Time, err error) {
	if title == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("empty title string")
	}

	// Extract date portion from title like "VIEQUES, PR, Monday, January 13, 2025 through Friday, December 19, 2025 UTC"
	// Remove location and timezone info, focus on the date range
	datePartRegex := regexp.MustCompile(`([A-Z][a-z]+day,\s+[A-Z][a-z]+\s+\d{1,2},\s+\d{4}(?:\s+through\s+[A-Z][a-z]+day,\s+[A-Z][a-z]+\s+\d{1,2},\s+\d{4})?)`)
	matches := datePartRegex.FindStringSubmatch(title)
	if len(matches) < 2 {
		return time.Time{}, time.Time{}, fmt.Errorf("no date pattern found in title: %s", title)
	}

	dateStr := matches[1]

	// Handle range formats like "Monday, January 13, 2025 through Friday, December 19, 2025"
	throughRegex := regexp.MustCompile(`(.+?)\s+through\s+(.+)`)
	if throughMatches := throughRegex.FindStringSubmatch(dateStr); len(throughMatches) == 3 {
		start, err1 := t.parseFlexibleDate(strings.TrimSpace(throughMatches[1]))
		end, err2 := t.parseFlexibleDate(strings.TrimSpace(throughMatches[2]))
		if err1 != nil || err2 != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("parsing date range: %v, %v", err1, err2)
		}
		return start, end, nil
	}

	// Handle single date
	single, err := t.parseFlexibleDate(strings.TrimSpace(dateStr))
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("parsing single date: %w", err)
	}
	return single, single.Add(24 * time.Hour), nil
}

// parseFlexibleDate attempts to parse various date formats
func (t *TFRClient) parseFlexibleDate(dateStr string) (time.Time, error) {
	formats := []string{
		"Monday, January 2, 2006",
		"January 2, 2006",
		"01/02/2006",
		"2006-01-02",
		"2006-01-02T15:04:05Z",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}

// calculatePolygonCenter calculates the centroid and approximate radius of a polygon
func (t *TFRClient) calculatePolygonCenter(coordinates [][]float64) (lat, lon, radius float64) {
	if len(coordinates) == 0 {
		return 0, 0, 0
	}

	// Convert Web Mercator coordinates to lat/lon and calculate centroid
	var latSum, lonSum float64
	var validPoints int

	for _, coord := range coordinates {
		if len(coord) >= 2 {
			mercatorLat, mercatorLon := coord[1], coord[0]
			lat, lon := t.webMercatorToWGS84(mercatorLat, mercatorLon)
			latSum += lat
			lonSum += lon
			validPoints++
		}
	}

	if validPoints == 0 {
		return 0, 0, 0
	}

	// Calculate centroid
	centerLat := latSum / float64(validPoints)
	centerLon := lonSum / float64(validPoints)

	// Calculate approximate radius as max distance from center to any vertex
	var maxDistance float64
	for _, coord := range coordinates {
		if len(coord) >= 2 {
			mercatorLat, mercatorLon := coord[1], coord[0]
			lat, lon := t.webMercatorToWGS84(mercatorLat, mercatorLon)
			distance := t.calculateDistance(centerLat, centerLon, lat, lon)
			if distance > maxDistance {
				maxDistance = distance
			}
		}
	}

	return centerLat, centerLon, maxDistance
}

// webMercatorToWGS84 converts Web Mercator (EPSG:3857) coordinates to WGS84 lat/lon
func (t *TFRClient) webMercatorToWGS84(mercatorY, mercatorX float64) (lat, lon float64) {
	// Convert from Web Mercator to WGS84
	lon = mercatorX / 20037508.34 * 180
	lat = mercatorY / 20037508.34 * 180
	lat = 180 / math.Pi * (2*math.Atan(math.Exp(lat*math.Pi/180)) - math.Pi/2)
	return lat, lon
}

// CheckTFRs checks for active TFRs in the area around the given coordinates
func (t *TFRClient) CheckTFRs(ctx context.Context, lat, lon float64) (*models.TFRCheck, error) {
	log.Printf("Checking TFRs around %.4f, %.4f within %d miles", lat, lon, t.config.SearchRadiusMiles)

	// Fetch active TFRs from FAA API
	allTFRs, err := t.fetchActiveTFRs(ctx)
	if err != nil {
		log.Printf("Failed to fetch TFRs: %v", err)
		// Return empty check when API fails
		return t.buildTFRCheck([]*models.TFR{}), err
	}

	// Filter TFRs that are currently active and within search area
	var activeTFRs []*models.TFR
	now := time.Now()

	for _, tfr := range allTFRs {
		// Check if TFR is currently active
		// Skip if TFR hasn't started yet OR if TFR has already ended
		if tfr.StartTime.After(now) || (!tfr.EndTime.IsZero() && tfr.EndTime.Before(now)) {
			continue // TFR is not currently active
		}

		// Check if TFR intersects with search area
		if t.isWithinSearchArea(lat, lon, tfr) {
			activeTFRs = append(activeTFRs, tfr)
		}
	}

	return t.buildTFRCheck(activeTFRs), nil
}

// buildTFRCheck creates a TFRCheck result from a list of active TFRs
func (t *TFRClient) buildTFRCheck(activeTFRs []*models.TFR) *models.TFRCheck {
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

	return check
}

// isWithinSearchArea checks if a TFR intersects with the search area around the given coordinates
func (t *TFRClient) isWithinSearchArea(homeLat, homeLon float64, tfr *models.TFR) bool {
	searchRadiusMiles := float64(t.config.SearchRadiusMiles)

	// Simple distance-based check
	if tfr.Latitude == 0 && tfr.Longitude == 0 {
		return false // No coordinate data available
	}

	// Distance between home location and TFR center
	distanceToCenter := t.calculateDistance(homeLat, homeLon, tfr.Latitude, tfr.Longitude)

	// Convert TFR radius from nautical miles to regular miles
	tfrRadiusMiles := tfr.Radius * 1.15078 // 1 nautical mile = 1.15078 miles

	// Check if circles intersect (distance between centers < sum of radii)
	return distanceToCenter <= (searchRadiusMiles + tfrRadiusMiles)
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
