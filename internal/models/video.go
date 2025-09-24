package models

import "time"

type Video struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	ChannelTitle    string    `json:"channel_title"`
	PublishedAt     time.Time `json:"published_at"`
	Duration        string    `json:"duration"`
	DurationSeconds int       `json:"duration_seconds"`
	ViewCount       int64     `json:"view_count"`
	URL             string    `json:"url"`
}

type Analysis struct {
	Video      *Video `json:"video"`
	IsRelevant bool   `json:"is_relevant"`
	Summary    string `json:"summary"`
	Reasoning  string `json:"reasoning"`
	ValueProp  string `json:"value_proposition"`
	Score      int    `json:"score"` // 1-10
}

type EmailReport struct {
	Date     time.Time   `json:"date"`
	Videos   []*Analysis `json:"videos"`
	Total    int         `json:"total_analyzed"`
	Selected int         `json:"selected"`
}
