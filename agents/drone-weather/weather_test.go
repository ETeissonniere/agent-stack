package droneweather

import (
	"testing"
	"time"

	"agent-stack/internal/models"
	"agent-stack/shared/config"
)

func TestAnalyzeWeatherConditions(t *testing.T) {
	client := &WeatherClient{
		config: &config.DroneWeatherConfig{
			MaxWindSpeedKmh:    25, // 25 km/h wind limit
			MinVisibilityKm:    5,  // 5 km visibility limit
			MaxPrecipitationMm: 0.0,
			MinTempC:           4.4,  // 4.4°C minimum temp
			MaxTempC:           35.0, // 35°C maximum temp
		},
	}

	tests := []struct {
		name          string
		weather       *models.WeatherData
		expectFlyable bool
		expectReasons int
	}{
		{
			name: "Perfect flying conditions",
			weather: &models.WeatherData{
				Temperature:   20.0, // 20°C
				WindSpeed:     14.4, // 14.4 km/h - light winds
				Visibility:    10.0, // 10 km - good visibility
				Precipitation: 0.0,
				Time:          time.Now(),
			},
			expectFlyable: true,
			expectReasons: 0,
		},
		{
			name: "Wind too high",
			weather: &models.WeatherData{
				Temperature:   20.0,
				WindSpeed:     28.8, // 28.8 km/h - over 25 km/h limit
				Visibility:    10.0,
				Precipitation: 0.0,
				Time:          time.Now(),
			},
			expectFlyable: false,
			expectReasons: 1,
		},
		{
			name: "Visibility too low",
			weather: &models.WeatherData{
				Temperature:   20.0,
				WindSpeed:     14.4, // 14.4 km/h - good wind speed
				Visibility:    2.0,  // 2.0 km - under 5 km limit
				Precipitation: 0.0,
				Time:          time.Now(),
			},
			expectFlyable: false,
			expectReasons: 1,
		},
		{
			name: "Precipitation present",
			weather: &models.WeatherData{
				Temperature:   20.0,
				WindSpeed:     14.4, // 14.4 km/h - good wind speed
				Visibility:    10.0,
				Precipitation: 1.0, // 1.0 mm precipitation present
				Time:          time.Now(),
			},
			expectFlyable: false,
			expectReasons: 1,
		},
		{
			name: "Temperature too cold",
			weather: &models.WeatherData{
				Temperature:   0.0,  // 0°C - under 4.4°C limit
				WindSpeed:     14.4, // 14.4 km/h - good wind speed
				Visibility:    10.0,
				Precipitation: 0.0,
				Time:          time.Now(),
			},
			expectFlyable: false,
			expectReasons: 1,
		},
		{
			name: "Temperature too hot",
			weather: &models.WeatherData{
				Temperature:   40.0, // 40°C - over 35°C limit
				WindSpeed:     14.4, // 14.4 km/h - good wind speed
				Visibility:    10.0,
				Precipitation: 0.0,
				Time:          time.Now(),
			},
			expectFlyable: false,
			expectReasons: 1,
		},
		{
			name: "Multiple issues",
			weather: &models.WeatherData{
				Temperature:   0.0,  // 0°C - too cold (under 4.4°C)
				WindSpeed:     36.0, // 36 km/h - too windy (over 25 km/h)
				Visibility:    1.0,  // 1 km - too low visibility (under 5 km)
				Precipitation: 2.0,  // 2.0 mm rain
				Time:          time.Now(),
			},
			expectFlyable: false,
			expectReasons: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := client.AnalyzeWeatherConditions(tt.weather)

			if analysis.IsFlyable != tt.expectFlyable {
				t.Errorf("Expected flyable=%v, got %v", tt.expectFlyable, analysis.IsFlyable)
			}

			if len(analysis.Reasons) != tt.expectReasons {
				t.Errorf("Expected %d reasons, got %d: %v", tt.expectReasons, len(analysis.Reasons), analysis.Reasons)
			}

			// Verify basic data consistency
			if tt.weather.WindSpeed < 0 {
				t.Error("Wind speed should not be negative")
			}
			if tt.weather.Temperature < -100 || tt.weather.Temperature > 100 {
				t.Errorf("Temperature seems unreasonable: %.1f°C", tt.weather.Temperature)
			}
			if tt.weather.Visibility < 0 {
				t.Error("Visibility should not be negative")
			}

			// Verify wind forecast is set
			if analysis.WindForecast == "" {
				t.Error("Wind forecast should not be empty")
			}
		})
	}
}

func TestBasicAnalysis(t *testing.T) {
	client := &WeatherClient{config: &config.DroneWeatherConfig{
		MaxWindSpeedKmh:    25, // 25 km/h limit
		MinVisibilityKm:    5,  // 5 km limit
		MaxPrecipitationMm: 0.0,
		MinTempC:           4.4,
		MaxTempC:           35.0,
	}}

	weather := &models.WeatherData{
		Temperature:   20.0, // 20°C - good temperature
		WindSpeed:     15.0, // 15 km/h - good wind speed
		Visibility:    10.0, // 10 km - good visibility
		Precipitation: 0.0,  // no precipitation
	}

	analysis := client.AnalyzeWeatherConditions(weather)

	// Should be flyable with these good conditions
	if !analysis.IsFlyable {
		t.Errorf("Expected flyable conditions, got not flyable with reasons: %v", analysis.Reasons)
	}

	// Should have no blocking reasons
	if len(analysis.Reasons) > 0 {
		t.Errorf("Expected no reasons for good conditions, got: %v", analysis.Reasons)
	}
}

func TestWindForecastGeneration(t *testing.T) {
	client := &WeatherClient{config: &config.DroneWeatherConfig{}}

	tests := []struct {
		name         string
		windSpeedKmh float64
		expectedText string
	}{
		{"Very light winds", 7.0, "Very light winds, excellent conditions"}, // 7 km/h - very light
		{"Light winds", 14.0, "Light winds, good conditions"},               // 14 km/h - light
		{"Moderate winds", 22.0, "Moderate winds, manageable"},              // 22 km/h - moderate
		{"Strong winds", 30.0, "Strong winds, challenging conditions"},      // 30 km/h - strong
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			weather := &models.WeatherData{
				WindSpeed:     tt.windSpeedKmh,
				Temperature:   20.0,
				Visibility:    10.0,
				Precipitation: 0.0,
			}

			analysis := client.AnalyzeWeatherConditions(weather)

			if analysis.WindForecast != tt.expectedText {
				t.Errorf("Expected wind forecast '%s', got '%s'", tt.expectedText, analysis.WindForecast)
			}
		})
	}
}
