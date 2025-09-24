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
			MaxWindSpeedMph:      15,
			MinVisibilityMiles:   3,
			MaxPrecipitationMm:   0.0,
			MinTempC:             4.4,  // 40°F
			MaxTempC:             35.0, // 95°F
		},
	}

	tests := []struct {
		name      string
		weather   *models.WeatherData
		expectFlyable bool
		expectReasons int
	}{
		{
			name: "Perfect flying conditions",
			weather: &models.WeatherData{
				Temperature:   20.0, // 68°F
				WindSpeed:     4.0,  // ~9 mph
				Visibility:    10.0, // ~6 miles
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
				WindSpeed:     8.0, // ~18 mph (over 15 mph limit)
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
				WindSpeed:     4.0,
				Visibility:    2.0, // ~1.2 miles (under 3 mile limit)
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
				WindSpeed:     4.0,
				Visibility:    10.0,
				Precipitation: 1.0, // Any precipitation
				Time:          time.Now(),
			},
			expectFlyable: false,
			expectReasons: 1,
		},
		{
			name: "Temperature too cold",
			weather: &models.WeatherData{
				Temperature:   0.0, // 32°F (under 40°F limit)
				WindSpeed:     4.0,
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
				Temperature:   40.0, // 104°F (over 95°F limit)
				WindSpeed:     4.0,
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
				Temperature:   0.0,  // Too cold
				WindSpeed:     10.0, // Too windy
				Visibility:    1.0,  // Too low visibility
				Precipitation: 2.0,  // Rain
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

			// Verify unit conversions are reasonable
			if analysis.WindSpeedMph < 0 {
				t.Error("Wind speed in mph should not be negative")
			}
			if analysis.TempF < -100 || analysis.TempF > 200 {
				t.Errorf("Temperature in F seems unreasonable: %.1f", analysis.TempF)
			}
			if analysis.VisibilityMi < 0 {
				t.Error("Visibility in miles should not be negative")
			}

			// Verify wind forecast is set
			if analysis.WindForecast == "" {
				t.Error("Wind forecast should not be empty")
			}
		})
	}
}

func TestUnitConversions(t *testing.T) {
	client := &WeatherClient{config: &config.DroneWeatherConfig{}}

	weather := &models.WeatherData{
		Temperature:   0.0,  // 0°C = 32°F
		WindSpeed:     10.0, // 10 m/s = 22.37 mph
		Visibility:    5.0,  // 5 km = 3.11 miles
	}

	analysis := client.AnalyzeWeatherConditions(weather)

	// Test temperature conversion (C to F)
	expectedTempF := 32.0
	if abs(analysis.TempF-expectedTempF) > 0.1 {
		t.Errorf("Temperature conversion: expected %.1f°F, got %.1f°F", expectedTempF, analysis.TempF)
	}

	// Test wind speed conversion (m/s to mph)
	expectedWindMph := 22.37
	if abs(analysis.WindSpeedMph-expectedWindMph) > 0.1 {
		t.Errorf("Wind speed conversion: expected %.2f mph, got %.2f mph", expectedWindMph, analysis.WindSpeedMph)
	}

	// Test visibility conversion (km to miles)
	expectedVisMi := 3.11
	if abs(analysis.VisibilityMi-expectedVisMi) > 0.1 {
		t.Errorf("Visibility conversion: expected %.2f miles, got %.2f miles", expectedVisMi, analysis.VisibilityMi)
	}
}

func TestWindForecastGeneration(t *testing.T) {
	client := &WeatherClient{config: &config.DroneWeatherConfig{}}

	tests := []struct {
		name         string
		windSpeedMs  float64
		expectedText string
	}{
		{"Very light winds", 2.0, "Very light winds, excellent conditions"},
		{"Light winds", 4.0, "Light winds, good conditions"},
		{"Moderate winds", 6.0, "Moderate winds, manageable"},
		{"Strong winds", 8.0, "Strong winds, challenging conditions"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			weather := &models.WeatherData{
				WindSpeed: tt.windSpeedMs,
				Temperature: 20.0,
				Visibility: 10.0,
				Precipitation: 0.0,
			}

			analysis := client.AnalyzeWeatherConditions(weather)

			if analysis.WindForecast != tt.expectedText {
				t.Errorf("Expected wind forecast '%s', got '%s'", tt.expectedText, analysis.WindForecast)
			}
		})
	}
}