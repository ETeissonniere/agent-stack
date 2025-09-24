package models

import "time"

// WeatherData represents current weather conditions from Open-Meteo API
type WeatherData struct {
	Latitude    float64   `json:"latitude"`
	Longitude   float64   `json:"longitude"`
	Temperature float64   `json:"temperature"`       // Celsius
	WindSpeed   float64   `json:"wind_speed"`        // m/s
	WindDir     int       `json:"wind_direction"`    // degrees
	Visibility  float64   `json:"visibility"`        // km
	Precipitation float64 `json:"precipitation"`     // mm
	Time        time.Time `json:"time"`
	Timezone    string    `json:"timezone"`          // IANA timezone (e.g., "America/Los_Angeles")
}

// WeatherAnalysis contains the analysis of weather conditions for drone flying
type WeatherAnalysis struct {
	Data          *WeatherData `json:"data"`
	IsFlyable     bool         `json:"is_flyable"`
	Reasons       []string     `json:"reasons"`
	WindSpeedMph  float64      `json:"wind_speed_mph"`
	TempF         float64      `json:"temp_f"`
	VisibilityMi  float64      `json:"visibility_mi"`
	BestWindow    string       `json:"best_window"`     // e.g., "9 AM - 4 PM"
	WindForecast  string       `json:"wind_forecast"`   // e.g., "Light and stable"
}