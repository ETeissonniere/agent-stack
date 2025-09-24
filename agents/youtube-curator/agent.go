package youtubecurator

import (
	"context"
	"fmt"
	"log"
	"time"

	"agent-stack/agents/youtube-curator/youtube"
	"agent-stack/internal/models"
	"agent-stack/shared/ai"
	"agent-stack/shared/config"
	"agent-stack/shared/email"
	"agent-stack/shared/scheduler"
	"agent-stack/shared/storage"
	"errors"
)

// YouTubeMetrics represents the metrics collected during a YouTube curation run
type YouTubeMetrics struct {
	VideosFound    int `json:"videos_found"`
	Analyzed       int `json:"analyzed"`
	Relevant       int `json:"relevant"`
	Skipped        int `json:"skipped"`
	AnalysisErrors int `json:"analysis_errors"`
}

// GetSummary implements the scheduler.Metrics interface
func (m YouTubeMetrics) GetSummary() string {
	return fmt.Sprintf("found %d videos, analyzed %d, selected %d relevant",
		m.VideosFound, m.Analyzed, m.Relevant)
}

// YouTubeAgent implements the scheduler.Agent interface
type YouTubeAgent struct {
	config             *config.Config
	youtubeClient      *youtube.Client
	analyzer           *ai.Analyzer
	emailSender        *email.Sender
	videoTracker       *storage.VideoTracker
	tokenRefreshTicker *time.Ticker
	tokenRefreshStop   chan bool
}

func NewYouTubeAgent(cfg *config.Config) *YouTubeAgent {
	return &YouTubeAgent{
		config: cfg,
	}
}

func (y *YouTubeAgent) Name() string {
	return "YouTube Curator"
}
func (y *YouTubeAgent) GetSchedule() string {
	return y.config.YouTubeCurator.Schedule
}

func (y *YouTubeAgent) Initialize() error {
	log.Printf("Initializing %s...", y.Name())

	if y.youtubeClient == nil {
		client, err := youtube.NewClient(&y.config.YouTubeCurator.YouTube)
		if err != nil {
			return fmt.Errorf("failed to create YouTube client: %w", err)
		}
		y.youtubeClient = client
		log.Println("YouTube client initialized")

		// Start background token refresher with configured interval
		refreshInterval := time.Duration(y.config.YouTubeCurator.YouTube.TokenRefreshMinutes) * time.Minute
		y.startTokenRefresher(refreshInterval)
	}

	if y.analyzer == nil {
		analyzer, err := ai.NewAnalyzer(y.config)
		if err != nil {
			return fmt.Errorf("failed to create AI analyzer: %w", err)
		}
		y.analyzer = analyzer
		log.Println("AI analyzer initialized")
	}

	if y.emailSender == nil {
		y.emailSender = email.NewSender(&y.config.Email)
		log.Println("Email sender initialized")
	}

	if y.videoTracker == nil {
		// Track videos for 7 days to avoid re-analyzing
		tracker, err := storage.NewVideoTracker("data", 7*24*time.Hour)
		if err != nil {
			return fmt.Errorf("failed to create video tracker: %w", err)
		}
		y.videoTracker = tracker
		log.Printf("Video tracker initialized (%d videos tracked)", tracker.GetAnalyzedCount())
	}

	return nil
}

// startTokenRefresher starts a background goroutine that refreshes the YouTube OAuth token periodically.
// This ensures the token stays fresh even during long periods of inactivity between scheduled runs.
// The refresher runs at the specified interval and saves refreshed tokens to disk automatically.
func (y *YouTubeAgent) startTokenRefresher(interval time.Duration) {
	if y.tokenRefreshTicker != nil {
		// Already running
		return
	}

	log.Printf("Starting background token refresher (interval: %v)", interval)
	y.tokenRefreshTicker = time.NewTicker(interval)
	y.tokenRefreshStop = make(chan bool)

	go func() {
		for {
			select {
			case <-y.tokenRefreshTicker.C:
				log.Println("Background token refresh triggered")
				if y.youtubeClient != nil {
					if err := y.youtubeClient.RefreshToken(); err != nil {
						log.Printf("Background token refresh failed: %v", err)
					} else {
						log.Println("Background token refresh successful")
					}
				} else {
					log.Println("Background token refresh skipped - client not initialized")
				}
			case <-y.tokenRefreshStop:
				log.Println("Stopping background token refresher")
				return
			}
		}
	}()
}

// StopTokenRefresher stops the background token refresh goroutine gracefully.
// This should be called when the application shuts down to ensure clean termination.
// It's safe to call multiple times or even if the refresher was never started.
func (y *YouTubeAgent) StopTokenRefresher() {
	if y.tokenRefreshTicker != nil {
		y.tokenRefreshTicker.Stop()
		if y.tokenRefreshStop != nil {
			y.tokenRefreshStop <- true
			close(y.tokenRefreshStop)
		}
		y.tokenRefreshTicker = nil
		y.tokenRefreshStop = nil
	}
}

func (y *YouTubeAgent) RunOnce(ctx context.Context, events *scheduler.AgentEvents) error {
	startTime := time.Now()

	// Proactively refresh token if needed before starting work
	if y.youtubeClient != nil {
		if err := y.youtubeClient.RefreshToken(); err != nil {
			log.Printf("Warning: Failed to refresh token: %v", err)
			// Continue anyway - the tokenSaver will auto-refresh on API calls
		}
	}

	// Fetch videos from subscriptions
	log.Println("Fetching videos from YouTube subscriptions...")
	videos, err := y.youtubeClient.GetSubscriptionVideos(ctx, 50)
	if err != nil {
		return fmt.Errorf("failed to get subscription videos: %w", err)
	}

	if len(videos) == 0 {
		log.Println("No new videos found")
		duration := time.Since(startTime)
		if events != nil && events.OnSuccess != nil {
			metrics := YouTubeMetrics{
				VideosFound:    0,
				Analyzed:       0,
				Relevant:       0,
				Skipped:        0,
				AnalysisErrors: 0,
			}
			events.OnSuccess(metrics, duration)
		}
		return nil
	}

	// Filter out already analyzed videos
	var newVideos []*models.Video
	var skippedCount int

	for _, video := range videos {
		if y.videoTracker.IsAnalyzed(video.ID) {
			skippedCount++
			continue
		}
		newVideos = append(newVideos, video)
	}

	if len(newVideos) == 0 {
		duration := time.Since(startTime)
		if events != nil && events.OnSuccess != nil {
			metrics := YouTubeMetrics{
				VideosFound:    len(videos),
				Analyzed:       0,
				Relevant:       0,
				Skipped:        skippedCount,
				AnalysisErrors: 0,
			}
			events.OnSuccess(metrics, duration)
		}
		return nil
	}

	var analyses []*models.Analysis
	var analysisErrors int
	var skippedShorts int
	var analyzedVideoIDs []string

	for i, video := range newVideos {
		log.Printf("Analyzing video %d/%d: %s", i+1, len(newVideos), video.Title)

		analysis, err := y.analyzer.AnalyzeVideo(ctx, video)
		if err != nil {
			if errors.Is(err, ai.ErrShortVideoSkipped) {
				skippedShorts++
				continue
			}
			analysisErrors++

			// Report individual analysis failure as partial (recoverable)
			if events != nil && events.OnPartialFailure != nil {
				events.OnPartialFailure(fmt.Errorf("failed to analyze video %s: %w", video.Title, err), time.Since(startTime))
			}

			if analysisErrors > len(newVideos)/2 {
				return fmt.Errorf("too many analysis failures (%d/%d), stopping", analysisErrors, i+1)
			}
			continue
		}

		analyses = append(analyses, analysis)
		analyzedVideoIDs = append(analyzedVideoIDs, video.ID)

		time.Sleep(2 * time.Second)
	}

	// Mark videos as analyzed (even if they weren't relevant)
	if len(analyzedVideoIDs) > 0 {
		if err := y.videoTracker.MarkMultipleAnalyzed(analyzedVideoIDs); err != nil {
			// Report video tracking failure as partial (doesn't affect core functionality)
			if events != nil && events.OnPartialFailure != nil {
				events.OnPartialFailure(fmt.Errorf("failed to mark videos as analyzed: %w", err), time.Since(startTime))
			}
		}
	}

	if analysisErrors > 0 {
		// Check if ALL videos failed to analyze (critical failure)
		if len(analyses) == 0 && len(newVideos) > 0 {
			// We had videos to analyze but ALL of them failed
			err := fmt.Errorf("all %d videos failed analysis - core functionality broken", len(newVideos))
			if events != nil && events.OnCriticalFailure != nil {
				events.OnCriticalFailure(err, time.Since(startTime))
			}
			return err
		} else {
			// Some videos failed, but we still processed others (partial failure)
			if events != nil && events.OnPartialFailure != nil {
				events.OnPartialFailure(fmt.Errorf("%d analysis errors occurred during processing", analysisErrors), time.Since(startTime))
			}
		}
	}

	// Filter relevant videos
	var relevantVideos []*models.Analysis
	for _, analysis := range analyses {
		if analysis.IsRelevant && analysis.Score >= 6 {
			relevantVideos = append(relevantVideos, analysis)
		}
	}

	// Send email report if there are relevant videos
	if len(relevantVideos) > 0 {
		report := &models.EmailReport{
			Date:     time.Now(),
			Videos:   relevantVideos,
			Total:    len(analyses),
			Selected: len(relevantVideos),
		}

		if err := y.emailSender.SendReport(report); err != nil {
			// Report email failure as CRITICAL - email delivery is core functionality
			if events != nil && events.OnCriticalFailure != nil {
				events.OnCriticalFailure(fmt.Errorf("failed to send email report: %w", err), time.Since(startTime))
			}
			return fmt.Errorf("failed to send email report: %w", err)
		}
	}

	// Record successful completion with detailed metrics
	duration := time.Since(startTime)
	if events != nil && events.OnSuccess != nil {
		metrics := YouTubeMetrics{
			VideosFound:    len(videos),
			Analyzed:       len(analyses),
			Relevant:       len(relevantVideos),
			Skipped:        skippedCount,
			AnalysisErrors: analysisErrors,
		}
		events.OnSuccess(metrics, duration)
	}

	log.Printf("Session complete: %d total videos, %d skipped (already analyzed), %d short videos skipped, %d analyzed, %d relevant",
		len(videos), skippedCount, skippedShorts, len(analyses), len(relevantVideos))

	return nil
}
