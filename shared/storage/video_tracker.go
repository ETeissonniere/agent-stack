package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// VideoTracker manages a persistent store of analyzed video IDs to prevent duplicate analysis
type VideoTracker struct {
	filePath    string
	analyzedIDs map[string]time.Time
	mu          sync.RWMutex
	maxAge      time.Duration
}

// TrackedVideo represents a video that has been analyzed
type TrackedVideo struct {
	VideoID    string    `json:"video_id"`
	AnalyzedAt time.Time `json:"analyzed_at"`
}

// NewVideoTracker creates a new video tracker with persistent storage
func NewVideoTracker(dataDir string, maxAge time.Duration) (*VideoTracker, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	filePath := filepath.Join(dataDir, "analyzed_videos.json")

	tracker := &VideoTracker{
		filePath:    filePath,
		analyzedIDs: make(map[string]time.Time),
		maxAge:      maxAge,
	}

	// Load existing data
	if err := tracker.load(); err != nil {
		return nil, fmt.Errorf("failed to load video tracker data: %w", err)
	}

	// Clean up old entries
	tracker.cleanup()

	return tracker, nil
}

// IsAnalyzed checks if a video ID has been analyzed recently
func (vt *VideoTracker) IsAnalyzed(videoID string) bool {
	vt.mu.RLock()
	defer vt.mu.RUnlock()

	analyzedAt, exists := vt.analyzedIDs[videoID]
	if !exists {
		return false
	}

	// Check if the analysis is still valid (not too old)
	return time.Since(analyzedAt) < vt.maxAge
}

// MarkAnalyzed marks a video ID as analyzed
func (vt *VideoTracker) MarkAnalyzed(videoID string) error {
	vt.mu.Lock()
	defer vt.mu.Unlock()

	vt.analyzedIDs[videoID] = time.Now()
	return vt.save()
}

// MarkMultipleAnalyzed marks multiple video IDs as analyzed in batch
func (vt *VideoTracker) MarkMultipleAnalyzed(videoIDs []string) error {
	vt.mu.Lock()
	defer vt.mu.Unlock()

	now := time.Now()
	for _, videoID := range videoIDs {
		vt.analyzedIDs[videoID] = now
	}
	return vt.save()
}

// GetAnalyzedCount returns the number of tracked videos
func (vt *VideoTracker) GetAnalyzedCount() int {
	vt.mu.RLock()
	defer vt.mu.RUnlock()
	return len(vt.analyzedIDs)
}

// Cleanup removes entries older than maxAge
func (vt *VideoTracker) cleanup() {
	cutoff := time.Now().Add(-vt.maxAge)

	for videoID, analyzedAt := range vt.analyzedIDs {
		if analyzedAt.Before(cutoff) {
			delete(vt.analyzedIDs, videoID)
		}
	}
}

// load reads the tracked videos from the JSON file
func (vt *VideoTracker) load() error {
	file, err := os.Open(vt.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet, start with empty tracker
			return nil
		}
		return fmt.Errorf("failed to open tracker file: %w", err)
	}
	defer file.Close()

	var trackedVideos []TrackedVideo
	if err := json.NewDecoder(file).Decode(&trackedVideos); err != nil {
		return fmt.Errorf("failed to decode tracker data: %w", err)
	}

	// Convert to map
	for _, tv := range trackedVideos {
		vt.analyzedIDs[tv.VideoID] = tv.AnalyzedAt
	}

	return nil
}

// save writes the tracked videos to the JSON file
func (vt *VideoTracker) save() error {
	// Convert map to slice for JSON serialization
	var trackedVideos []TrackedVideo
	for videoID, analyzedAt := range vt.analyzedIDs {
		trackedVideos = append(trackedVideos, TrackedVideo{
			VideoID:    videoID,
			AnalyzedAt: analyzedAt,
		})
	}

	file, err := os.Create(vt.filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(trackedVideos)
}
