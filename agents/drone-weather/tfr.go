package droneweather

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

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

// parseCoordinatePair parses a simple decimal coordinate pair "lat, lon"
func parseCoordinatePair(coordPair string) (lat, lon float64, err error) {
	parts := strings.Split(coordPair, ",")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid coordinate pair format: %s", coordPair)
	}

	lat, err = strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid latitude: %w", err)
	}

	lon, err = strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid longitude: %w", err)
	}

	return lat, lon, nil
}

// TFR fetching and parsing functions

// fetchActiveTFRs fetches the list of active TFRs from FAA website
func (t *TFRClient) fetchActiveTFRs(ctx context.Context) ([]*models.TFR, error) {
	log.Printf("Fetching fresh TFR data")

	// Use the current FAA TFR list endpoint (tfr3 is the newer version that FAA redirects to)
	endpoint := "https://tfr.faa.gov/tfr3/?page=list"
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

	// Parse based on content type
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		return t.parseJSONTFRs(resp.Body)
	}

	// Default to HTML parsing
	return t.parseHTMLTFRs(resp.Body)
}

// parseHTMLTFRs parses TFR data from HTML content
func (t *TFRClient) parseHTMLTFRs(body io.Reader) ([]*models.TFR, error) {
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return nil, fmt.Errorf("parsing HTML: %w", err)
	}

	var tfrs []*models.TFR

	// Look for table rows with TFR data
	doc.Find("tr").Each(func(i int, s *goquery.Selection) {
		// Skip header row
		if i == 0 {
			return
		}

		cells := s.Find("td")
		if cells.Length() < 5 {
			return // Not enough columns for TFR data
		}

		tfr := &models.TFR{}

		// Parse common table formats (adjust based on actual FAA table structure)
		cells.Each(func(j int, cell *goquery.Selection) {
			text := strings.TrimSpace(cell.Text())
			switch j {
			case 0: // Date
				if date, err := time.Parse("01/02/2006", text); err == nil {
					tfr.StartTime = date
				}
			case 1: // NOTAM
				tfr.ID = text
			case 2: // Facility
				tfr.Name = text
			case 3: // State
				// Could be incorporated into name or reason
			case 4: // Type
				tfr.Type = text
			case 5: // Description
				tfr.Reason = text
			}
		})

		// Set default values for coordinates and radius
		if tfr.Latitude == 0 && tfr.Longitude == 0 {
			// Try to parse coordinates from the description if available
			if tfr.Reason != "" {
				if lat, lon, radius, found := t.parseSimpleCoordinates(tfr.Reason); found {
					tfr.Latitude = lat
					tfr.Longitude = lon
					tfr.Radius = radius
				}
			}
		}

		// Only add if we have basic info
		if tfr.ID != "" && tfr.Type != "" {
			tfrs = append(tfrs, tfr)
		}
	})

	return tfrs, nil
}

// parseJSONTFRs parses TFR data from JSON content (not implemented)
func (t *TFRClient) parseJSONTFRs(body io.Reader) ([]*models.TFR, error) {
	log.Printf("JSON TFR parsing not implemented, FAA primarily uses HTML")
	return []*models.TFR{}, nil
}

// parseSimpleCoordinates attempts to extract basic coordinate and radius info from text
func (t *TFRClient) parseSimpleCoordinates(text string) (lat, lon, radius float64, found bool) {
	// Look for patterns like "within X miles of lat, lon" or "centered at lat, lon"
	radiusRegex := regexp.MustCompile(`(?i)(?:within\s+a?\s*(\d+(?:\.\d+)?)[- ]?(?:nautical\s+)?miles?|radius\s+(\d+(?:\.\d+)?)\s*(?:nautical\s+)?miles?)`)
	coordRegex := regexp.MustCompile(`(-?\d+\.?\d*),\s*(-?\d+\.?\d*)`)

	radiusMatches := radiusRegex.FindStringSubmatch(text)
	coordMatches := coordRegex.FindStringSubmatch(text)

	if len(coordMatches) >= 3 {
		var err error
		lat, err = strconv.ParseFloat(coordMatches[1], 64)
		if err != nil {
			return 0, 0, 0, false
		}

		lon, err = strconv.ParseFloat(coordMatches[2], 64)
		if err != nil {
			return 0, 0, 0, false
		}

		// Try to get radius
		if len(radiusMatches) >= 2 {
			var radiusStr string
			if radiusMatches[1] != "" {
				radiusStr = radiusMatches[1]
			} else {
				radiusStr = radiusMatches[2]
			}

			if r, err := strconv.ParseFloat(radiusStr, 64); err == nil {
				radius = r
			} else {
				radius = 10 // default 10 nautical miles
			}
		} else {
			radius = 10 // default 10 nautical miles
		}

		return lat, lon, radius, true
	}

	return 0, 0, 0, false
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
		if tfr.StartTime.After(now) || tfr.EndTime.Before(now) {
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