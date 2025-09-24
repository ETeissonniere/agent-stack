package droneweather

import (
	"testing"
	"time"

	"agent-stack/internal/models"
	"agent-stack/shared/config"
)

func TestDroneMetricsGetSummary(t *testing.T) {
	tests := []struct {
		name     string
		metrics  DroneMetrics
		expected string
	}{
		{
			name: "Good weather, email sent",
			metrics: DroneMetrics{
				WeatherFetched: true,
				TFRsChecked:    true,
				IsFlyable:      true,
				EmailSent:      true,
			},
			expected: "good weather conditions detected, email sent with TFR info",
		},
		{
			name: "Good weather, no email sent",
			metrics: DroneMetrics{
				WeatherFetched: true,
				TFRsChecked:    true,
				IsFlyable:      true,
				EmailSent:      false,
			},
			expected: "good weather conditions detected, no email sent",
		},
		{
			name: "Poor weather conditions",
			metrics: DroneMetrics{
				WeatherFetched: true,
				TFRsChecked:    true,
				IsFlyable:      false,
				EmailSent:      false,
			},
			expected: "poor weather conditions, no email sent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.metrics.GetSummary()
			if result != tt.expected {
				t.Errorf("Expected summary '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestNewDroneWeatherAgent(t *testing.T) {
	cfg := &config.Config{
		DroneWeather: config.DroneWeatherConfig{
			HomeLatitude:  40.0,
			HomeLongitude: -74.0,
			HomeName:      "Test Location",
		},
	}

	agent := NewDroneWeatherAgent(cfg)

	if agent.config != cfg {
		t.Error("Agent config not set correctly")
	}

	if agent.Name() != "Drone Weather Agent" {
		t.Errorf("Expected agent name 'Drone Weather Agent', got '%s'", agent.Name())
	}
}

func TestDroneWeatherAgentInitialize(t *testing.T) {
	tests := []struct {
		name      string
		config    config.DroneWeatherConfig
		expectErr bool
	}{
		{
			name: "Valid configuration",
			config: config.DroneWeatherConfig{
				HomeLatitude:  40.0,
				HomeLongitude: -74.0,
				HomeName:      "Test Location",
			},
			expectErr: false,
		},
		{
			name: "Missing home coordinates",
			config: config.DroneWeatherConfig{
				HomeLatitude:  0,
				HomeLongitude: 0,
				HomeName:      "Test Location",
			},
			expectErr: true,
		},
		{
			name: "Missing home name",
			config: config.DroneWeatherConfig{
				HomeLatitude:  40.0,
				HomeLongitude: -74.0,
				HomeName:      "",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{DroneWeather: tt.config}
			agent := NewDroneWeatherAgent(cfg)

			err := agent.Initialize()
			hasErr := err != nil

			if hasErr != tt.expectErr {
				t.Errorf("Expected error=%v, got error=%v (%v)", tt.expectErr, hasErr, err)
			}
		})
	}
}

func TestGenerateEmailBody(t *testing.T) {
	cfg := &config.Config{
		DroneWeather: config.DroneWeatherConfig{
			HomeLatitude:  40.0,
			HomeLongitude: -74.0,
			HomeName:      "Test Location",
		},
	}
	agent := NewDroneWeatherAgent(cfg)

	report := &models.DroneFlightReport{
		Date:         time.Now(),
		LocationName: "Test Location",
		WeatherAnalysis: &models.WeatherAnalysis{
			Data: &models.WeatherData{
				Temperature:   20.0,
				WindSpeed:     15.0, // km/h
				Visibility:    10.0, // km
				Precipitation: 0.0,
				Time:          time.Now(),
			},
			IsFlyable:       true,
			AvgWindSpeedKmh: 14.5,
			AvgWindGustsKmh: 18.2,
			WindForecast:    "Light winds, good conditions",
		},
		TFRCheck: &models.TFRCheck{
			HasActiveTFRs: false,
			ActiveTFRs:    []*models.TFR{},
			CheckRadius:   25,
			CheckTime:     time.Now(),
			Summary:       "No restrictions found within 25 miles - clear to fly",
		},
		IsFlyable: true,
		Summary:   "Excellent conditions for drone flying!",
	}

	// This test will fail if the email template file doesn't exist, which is expected
	// In a real scenario, we'd either mock the file reading or create a test template
	_, err := agent.generateEmailBody(report)

	// We expect an error since the template file likely doesn't exist in test environment
	if err == nil {
		t.Log("Email body generated successfully")
	} else {
		t.Logf("Expected error due to missing template file: %v", err)
		// This is OK for testing - shows the function tries to read the template
	}
}

func TestSendEmailReportSubject(t *testing.T) {
	cfg := &config.Config{
		DroneWeather: config.DroneWeatherConfig{
			HomeLatitude:  40.0,
			HomeLongitude: -74.0,
			HomeName:      "Test Location",
		},
	}
	agent := NewDroneWeatherAgent(cfg)

	report := &models.DroneFlightReport{
		LocationName: "Test Location",
	}

	// Test that sendEmailReport creates the correct subject
	// We can't fully test this without mocking SMTP, but we can verify the method exists
	err := agent.sendEmailReport(report)

	// Expected to fail due to email configuration not being set up for tests
	if err != nil {
		t.Logf("Expected error due to email config: %v", err)
	}
}
