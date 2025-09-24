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
	Hourly struct {
		Time      []string  `json:"time"`
		WindSpeed []float64 `json:"wind_speed_10m"`
		WindGusts []float64 `json:"wind_gusts_10m"`
	} `json:"hourly"`
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
	url := fmt.Sprintf("%s?latitude=%.4f&longitude=%.4f&current=temperature_2m,wind_speed_10m,wind_direction_10m,visibility,precipitation&hourly=wind_speed_10m,wind_gusts_10m&wind_speed_unit=kmh&temperature_unit=celsius&timezone=auto&forecast_hours=24",
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

	// Parse hourly data
	var hourlyData *models.HourlyForecast
	if len(apiResp.Hourly.Time) > 0 && len(apiResp.Hourly.WindSpeed) > 0 && len(apiResp.Hourly.WindGusts) > 0 {
		hourlyData = &models.HourlyForecast{
			Times:      make([]time.Time, len(apiResp.Hourly.Time)),
			WindSpeeds: apiResp.Hourly.WindSpeed,
			WindGusts:  apiResp.Hourly.WindGusts,
		}

		// Parse hourly timestamps
		for i, timeStr := range apiResp.Hourly.Time {
			parsedHourlyTime, err := time.ParseInLocation("2006-01-02T15:04", timeStr, location)
			if err != nil {
				log.Printf("Warning: Failed to parse hourly time %s: %v", timeStr, err)
				continue
			}
			hourlyData.Times[i] = parsedHourlyTime
		}
	}

	return &models.WeatherData{
		Latitude:      apiResp.Latitude,
		Longitude:     apiResp.Longitude,
		Temperature:   apiResp.Current.Temperature,
		WindSpeed:     apiResp.Current.WindSpeed, // Now in km/h from API
		WindDir:       apiResp.Current.WindDirection,
		Visibility:    apiResp.Current.Visibility / 1000, // Convert m to km
		Precipitation: apiResp.Current.Precipitation,
		Time:          parsedTime,
		Timezone:      apiResp.Timezone,
		HourlyData:    hourlyData,
	}, nil
}

// AnalyzeWeatherConditions analyzes weather data against flying thresholds
func (w *WeatherClient) AnalyzeWeatherConditions(data *models.WeatherData) *models.WeatherAnalysis {
	analysis := &models.WeatherAnalysis{
		Data:         data,
		IsFlyable:    true,
		Reasons:      []string{},
		WindForecast: "Light and stable through afternoon", // Simplified forecast
	}

	// Calculate average wind values from hourly data
	if data.HourlyData != nil && len(data.HourlyData.WindSpeeds) > 0 {
		// Calculate average wind speed
		var totalWindSpeed float64
		for _, speed := range data.HourlyData.WindSpeeds {
			totalWindSpeed += speed
		}
		analysis.AvgWindSpeedKmh = totalWindSpeed / float64(len(data.HourlyData.WindSpeeds))

		// Calculate average wind gusts
		if len(data.HourlyData.WindGusts) > 0 {
			var totalGusts float64
			for _, gust := range data.HourlyData.WindGusts {
				totalGusts += gust
			}
			analysis.AvgWindGustsKmh = totalGusts / float64(len(data.HourlyData.WindGusts))
		}
	}

	// Check wind speed
	if data.WindSpeed > float64(w.config.MaxWindSpeedKmh) {
		analysis.IsFlyable = false
		analysis.Reasons = append(analysis.Reasons, fmt.Sprintf("Wind speed too high: %.1f km/h (max: %d km/h)", data.WindSpeed, w.config.MaxWindSpeedKmh))
	}

	// Check visibility
	if data.Visibility < float64(w.config.MinVisibilityKm) {
		analysis.IsFlyable = false
		analysis.Reasons = append(analysis.Reasons, fmt.Sprintf("Visibility too low: %.1f km (min: %d km)", data.Visibility, w.config.MinVisibilityKm))
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

	// Update wind forecast based on conditions (using km/h)
	if data.WindSpeed < 8 { // ~5 mph
		analysis.WindForecast = "Very light winds, excellent conditions"
	} else if data.WindSpeed < 16 { // ~10 mph
		analysis.WindForecast = "Light winds, good conditions"
	} else if data.WindSpeed < 24 { // ~15 mph
		analysis.WindForecast = "Moderate winds, manageable"
	} else {
		analysis.WindForecast = "Strong winds, challenging conditions"
	}

	return analysis
}
