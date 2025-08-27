package youtubecurator

import (
	"context"
	"fmt"
	"log"
	"time"

	"agent-stack/internal/models"
	"agent-stack/shared/ai"
	"agent-stack/shared/config"
	"agent-stack/shared/email"
	"agent-stack/shared/scheduler"
	"agent-stack/shared/storage"
	"agent-stack/agents/youtube-curator/youtube"
    "errors"
)

// YouTubeMetrics represents the metrics collected during a YouTube curation run
type YouTubeMetrics struct {
	VideosFound     int `json:"videos_found"`
	Analyzed        int `json:"analyzed"`
	Relevant        int `json:"relevant"`
	Skipped         int `json:"skipped"`
	AnalysisErrors  int `json:"analysis_errors"`
}

// GetSummary implements the scheduler.Metrics interface
func (m YouTubeMetrics) GetSummary() string {
	return fmt.Sprintf("found %d videos, analyzed %d, selected %d relevant", 
		m.VideosFound, m.Analyzed, m.Relevant)
}

// YouTubeAgent implements the scheduler.Agent interface
type YouTubeAgent struct {
    config        *config.Config
    youtubeClient *youtube.Client
    analyzer      *ai.Analyzer
    emailSender   *email.Sender
    videoTracker  *storage.VideoTracker
}

func NewYouTubeAgent(cfg *config.Config) *YouTubeAgent {
    return &YouTubeAgent{
        config: cfg,
    }
}

func (y *YouTubeAgent) Name() string {
	return "YouTube Curator"
}

func (y *YouTubeAgent) Initialize() error {
    log.Printf("Initializing %s...", y.Name())
	
    if y.youtubeClient == nil {
        client, err := youtube.NewClient(&y.config.YouTube)
		if err != nil {
			return fmt.Errorf("failed to create YouTube client: %w", err)
		}
		y.youtubeClient = client
		log.Println("YouTube client initialized")
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

func (y *YouTubeAgent) RunOnce(ctx context.Context, events *scheduler.AgentEvents) error {
	startTime := time.Now()
	
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
				VideosFound: 0,
				Analyzed: 0,
				Relevant: 0,
				Skipped: 0,
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

	log.Printf("Found %d videos (%d new, %d already analyzed)", len(videos), len(newVideos), skippedCount)

	if len(newVideos) == 0 {
		log.Println("No new videos to analyze")
		duration := time.Since(startTime)
		if events != nil && events.OnSuccess != nil {
			metrics := YouTubeMetrics{
				VideosFound: len(videos),
				Analyzed: 0,
				Relevant: 0,
				Skipped: skippedCount,
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
			log.Printf("Warning: Failed to analyze video %s (%s): %v", video.ID, video.Title, err)
			analysisErrors++
			
			// Report individual analysis failure
			if events != nil && events.OnFailure != nil {
				events.OnFailure(fmt.Errorf("failed to analyze video %s: %w", video.Title, err), time.Since(startTime))
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
			log.Printf("Warning: Failed to mark videos as analyzed: %v", err)
			// Report video tracking failure
			if events != nil && events.OnFailure != nil {
				events.OnFailure(fmt.Errorf("failed to mark videos as analyzed: %w", err), time.Since(startTime))
			}
		}
	}

	if analysisErrors > 0 {
		log.Printf("Analysis completed with %d errors", analysisErrors)
		// Report summary of analysis errors
		if events != nil && events.OnFailure != nil {
			events.OnFailure(fmt.Errorf("%d analysis errors occurred during processing", analysisErrors), time.Since(startTime))
		}
	}

	// Filter relevant videos
	var relevantVideos []*models.Analysis
	for _, analysis := range analyses {
		if analysis.IsRelevant && analysis.Score >= 6 {
			relevantVideos = append(relevantVideos, analysis)
		}
	}

	log.Printf("Analysis complete: %d total, %d relevant (%d short videos skipped)", len(analyses), len(relevantVideos), skippedShorts)

	// Send email report if there are relevant videos
	if len(relevantVideos) > 0 {
		report := &models.EmailReport{
			Date:     time.Now(),
			Videos:   relevantVideos,
			Total:    len(analyses),
			Selected: len(relevantVideos),
		}

		log.Printf("Sending email report with %d videos", len(relevantVideos))
		if err := y.emailSender.SendReport(report); err != nil {
			// Report email failure as partial failure, don't stop execution
			if events != nil && events.OnFailure != nil {
				events.OnFailure(fmt.Errorf("failed to send email report: %w", err), time.Since(startTime))
			}
			log.Printf("Warning: Failed to send email report: %v", err)
		} else {
			log.Println("Email report sent successfully")
		}
		log.Println("Email report sent successfully")
	} else {
		log.Println("No relevant videos found, skipping email")
	}

	// Record successful completion with detailed metrics
	duration := time.Since(startTime)
	if events != nil && events.OnSuccess != nil {
		metrics := YouTubeMetrics{
			VideosFound: len(videos),
			Analyzed: len(analyses),
			Relevant: len(relevantVideos),
			Skipped: skippedCount,
			AnalysisErrors: analysisErrors,
		}
		events.OnSuccess(metrics, duration)
	}
	
	log.Printf("Session complete: %d total videos, %d skipped (already analyzed), %d short videos skipped, %d analyzed, %d relevant", 
		len(videos), skippedCount, skippedShorts, len(analyses), len(relevantVideos))

	return nil
}
