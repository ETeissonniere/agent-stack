package droneweather

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"agent-stack/internal/models"
	"agent-stack/shared/config"
)

// WeatherClient handles interactions with the Open-Meteo API
type WeatherClient struct {
	config *config.DroneWeatherConfig
	client *http.Client
}

// OpenMeteoResponse represents the response from Open-Meteo API
type OpenMeteoResponse struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timezone  string  `json:"timezone"`
	Current   struct {
		Time          string  `json:"time"`
		Temperature   float64 `json:"temperature_2m"`
		WindSpeed     float64 `json:"wind_speed_10m"`
		WindDirection int     `json:"wind_direction_10m"`
		Visibility    float64 `json:"visibility"`
		Precipitation float64 `json:"precipitation"`
	} `json:"current"`
}

func NewWeatherClient(cfg *config.DroneWeatherConfig) *WeatherClient {
	return &WeatherClient{
		config: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetCurrentWeather fetches current weather data from Open-Meteo API
func (w *WeatherClient) GetCurrentWeather(ctx context.Context, lat, lon float64) (*models.WeatherData, error) {
	url := fmt.Sprintf("%s?latitude=%.4f&longitude=%.4f&current=temperature_2m,wind_speed_10m,wind_direction_10m,visibility,precipitation&wind_speed_unit=ms&temperature_unit=celsius&timezone=auto",
		w.config.WeatherURL, lat, lon)

	log.Printf("Fetching weather data from: %s", url)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create weather request: %w", err)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch weather data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather API returned status %d", resp.StatusCode)
	}

	var apiResp OpenMeteoResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode weather response: %w", err)
	}

	// Parse time with timezone
	location, err := time.LoadLocation(apiResp.Timezone)
	if err != nil {
		log.Printf("Warning: Failed to load timezone %s, using UTC: %v", apiResp.Timezone, err)
		location = time.UTC
	}

	parsedTime, err := time.ParseInLocation("2006-01-02T15:04", apiResp.Current.Time, location)
	if err != nil {
		return nil, fmt.Errorf("failed to parse weather time: %w", err)
	}

	return &models.WeatherData{
		Latitude:      apiResp.Latitude,
		Longitude:     apiResp.Longitude,
		Temperature:   apiResp.Current.Temperature,
		WindSpeed:     apiResp.Current.WindSpeed,
		WindDir:       apiResp.Current.WindDirection,
		Visibility:    apiResp.Current.Visibility / 1000, // Convert m to km
		Precipitation: apiResp.Current.Precipitation,
		Time:          parsedTime,
		Timezone:      apiResp.Timezone,
	}, nil
}

// AnalyzeWeatherConditions analyzes weather data against flying thresholds
func (w *WeatherClient) AnalyzeWeatherConditions(data *models.WeatherData) *models.WeatherAnalysis {
	analysis := &models.WeatherAnalysis{
		Data:         data,
		IsFlyable:    true,
		Reasons:      []string{},
		WindSpeedMph: data.WindSpeed * 2.237,                // m/s to mph
		TempF:        (data.Temperature * 9 / 5) + 32,       // C to F (for display)
		VisibilityMi: data.Visibility * 0.621371,            // km to miles
		BestWindow:   "9 AM - 4 PM",                         // Default flying window
		WindForecast: "Light and stable through afternoon",  // Simplified forecast
	}

	// Check wind speed
	if analysis.WindSpeedMph > float64(w.config.MaxWindSpeedMph) {
		analysis.IsFlyable = false
		analysis.Reasons = append(analysis.Reasons, fmt.Sprintf("Wind speed too high: %.1f mph (max: %d mph)", analysis.WindSpeedMph, w.config.MaxWindSpeedMph))
	}

	// Check visibility
	if analysis.VisibilityMi < float64(w.config.MinVisibilityMiles) {
		analysis.IsFlyable = false
		analysis.Reasons = append(analysis.Reasons, fmt.Sprintf("Visibility too low: %.1f miles (min: %d miles)", analysis.VisibilityMi, w.config.MinVisibilityMiles))
	}

	// Check precipitation
	if data.Precipitation > w.config.MaxPrecipitationMm {
		analysis.IsFlyable = false
		analysis.Reasons = append(analysis.Reasons, fmt.Sprintf("Precipitation present: %.1f mm (max: %.1f mm)", data.Precipitation, w.config.MaxPrecipitationMm))
	}

	// Check temperature (use Celsius for comparisons)
	if data.Temperature < w.config.MinTempC {
		analysis.IsFlyable = false
		analysis.Reasons = append(analysis.Reasons, fmt.Sprintf("Temperature too low: %.1f°C (min: %.1f°C)", data.Temperature, w.config.MinTempC))
	}

	if data.Temperature > w.config.MaxTempC {
		analysis.IsFlyable = false
		analysis.Reasons = append(analysis.Reasons, fmt.Sprintf("Temperature too high: %.1f°C (max: %.1f°C)", data.Temperature, w.config.MaxTempC))
	}

	// Update wind forecast based on conditions
	if analysis.WindSpeedMph < 5 {
		analysis.WindForecast = "Very light winds, excellent conditions"
	} else if analysis.WindSpeedMph < 10 {
		analysis.WindForecast = "Light winds, good conditions"
	} else if analysis.WindSpeedMph < 15 {
		analysis.WindForecast = "Moderate winds, manageable"
	} else {
		analysis.WindForecast = "Strong winds, challenging conditions"
	}

	return analysis
}