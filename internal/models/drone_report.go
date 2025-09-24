package models

import "time"

// DroneFlightReport represents a drone weather report for email delivery
type DroneFlightReport struct {
	Date            time.Time        `json:"date"`
	LocationName    string           `json:"location_name"`
	WeatherAnalysis *WeatherAnalysis `json:"weather_analysis"`
	TFRCheck        *TFRCheck        `json:"tfr_check"`
	IsFlyable       bool             `json:"is_flyable"`
	Summary         string           `json:"summary"`
}