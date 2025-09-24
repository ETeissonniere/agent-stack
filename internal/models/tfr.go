package models

import "time"

// TFR represents a Temporary Flight Restriction from FAA API
type TFR struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	Radius    float64   `json:"radius"`    // nautical miles
	AltMin    int       `json:"alt_min"`   // feet
	AltMax    int       `json:"alt_max"`   // feet
	Reason    string    `json:"reason"`
}

// TFRCheck contains the results of checking for TFRs in the area
type TFRCheck struct {
	HasActiveTFRs bool   `json:"has_active_tfrs"`
	ActiveTFRs    []*TFR `json:"active_tfrs"`
	CheckRadius   int    `json:"check_radius"`  // miles
	CheckTime     time.Time `json:"check_time"`
	Summary       string `json:"summary"`       // e.g., "None active within 25 miles"
}