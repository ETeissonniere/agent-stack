package models

import "time"

// HourlyForecast represents hourly weather forecast data
type HourlyForecast struct {
	Times      []time.Time `json:"times"`
	WindSpeeds []float64   `json:"wind_speeds"`  // km/h
	WindGusts  []float64   `json:"wind_gusts"`   // km/h
}

// WeatherData represents current weather conditions from Open-Meteo API
type WeatherData struct {
	Latitude      float64          `json:"latitude"`
	Longitude     float64          `json:"longitude"`
	Temperature   float64          `json:"temperature"`       // Celsius
	WindSpeed     float64          `json:"wind_speed"`        // km/h (changed from m/s)
	WindDir       int              `json:"wind_direction"`    // degrees
	Visibility    float64          `json:"visibility"`        // km
	Precipitation float64          `json:"precipitation"`     // mm
	Time          time.Time        `json:"time"`
	Timezone      string           `json:"timezone"`          // IANA timezone (e.g., "America/Los_Angeles")
	HourlyData    *HourlyForecast  `json:"hourly_data,omitempty"` // Hourly forecast data
}

// WeatherAnalysis contains the analysis of weather conditions for drone flying
type WeatherAnalysis struct {
	Data            *WeatherData `json:"data"`
	IsFlyable       bool         `json:"is_flyable"`
	Reasons         []string     `json:"reasons"`
	AvgWindSpeedKmh float64      `json:"avg_wind_speed_kmh"` // Average wind speed over 24h forecast
	AvgWindGustsKmh float64      `json:"avg_wind_gusts_kmh"` // Average wind gusts over 24h forecast
	WindForecast    string       `json:"wind_forecast"`      // e.g., "Light and stable"
}