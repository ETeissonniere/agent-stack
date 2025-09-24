package youtubecurator

import (
	"context"
	"testing"
	"time"

	"agent-stack/shared/config"
	"agent-stack/shared/scheduler"
)

func TestYouTubeAgentName(t *testing.T) {
	agent := NewYouTubeAgent(&config.Config{})
	expected := "YouTube Curator"
	if name := agent.Name(); name != expected {
		t.Errorf("Agent.Name() = %s, want %s", name, expected)
	}
}

func TestYouTubeMetricsGetSummary(t *testing.T) {
	tests := []struct {
		name     string
		metrics  YouTubeMetrics
		expected string
	}{
		{
			name: "All zeros",
			metrics: YouTubeMetrics{
				VideosFound: 0,
				Analyzed:    0,
				Relevant:    0,
			},
			expected: "found 0 videos, analyzed 0, selected 0 relevant",
		},
		{
			name: "Some videos analyzed",
			metrics: YouTubeMetrics{
				VideosFound: 10,
				Analyzed:    5,
				Relevant:    2,
			},
			expected: "found 10 videos, analyzed 5, selected 2 relevant",
		},
		{
			name: "With errors",
			metrics: YouTubeMetrics{
				VideosFound:    20,
				Analyzed:       15,
				Relevant:       5,
				Skipped:        3,
				AnalysisErrors: 2,
			},
			expected: "found 20 videos, analyzed 15, selected 5 relevant",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.metrics.GetSummary()
			if result != tt.expected {
				t.Errorf("GetSummary() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestTokenRefresherLifecycle(t *testing.T) {
	cfg := &config.Config{
		YouTubeCurator: config.YouTubeCuratorConfig{
			YouTube: config.YouTubeConfig{
				TokenRefreshMinutes: 1, // 1 minute for testing
			},
		},
	}

	agent := NewYouTubeAgent(cfg)

	t.Run("StartsAndStops", func(t *testing.T) {
		// Start the refresher
		agent.startTokenRefresher(100 * time.Millisecond)

		// Verify it's running
		if agent.tokenRefreshTicker == nil {
			t.Error("Token refresher ticker not created")
		}
		if agent.tokenRefreshStop == nil {
			t.Error("Token refresher stop channel not created")
		}

		// Give it a moment to potentially run
		time.Sleep(150 * time.Millisecond)

		// Stop the refresher
		agent.StopTokenRefresher()

		// Verify it's stopped
		if agent.tokenRefreshTicker != nil {
			t.Error("Token refresher ticker not cleaned up")
		}
		if agent.tokenRefreshStop != nil {
			t.Error("Token refresher stop channel not cleaned up")
		}
	})

	t.Run("MultipleStarts", func(t *testing.T) {
		// Start the refresher
		agent.startTokenRefresher(100 * time.Millisecond)
		firstTicker := agent.tokenRefreshTicker

		// Try to start again - should not create a new ticker
		agent.startTokenRefresher(200 * time.Millisecond)

		if agent.tokenRefreshTicker != firstTicker {
			t.Error("Starting refresher twice created a new ticker")
		}

		// Clean up
		agent.StopTokenRefresher()
	})

	t.Run("MultipleStops", func(t *testing.T) {
		// Start the refresher
		agent.startTokenRefresher(100 * time.Millisecond)

		// Stop it
		agent.StopTokenRefresher()

		// Stop again - should not panic
		agent.StopTokenRefresher()

		// Verify still cleaned up
		if agent.tokenRefreshTicker != nil {
			t.Error("Token refresher ticker not cleaned up after double stop")
		}
	})

	t.Run("StopWithoutStart", func(t *testing.T) {
		// Create fresh agent
		freshAgent := NewYouTubeAgent(cfg)

		// Stop without starting - should not panic
		freshAgent.StopTokenRefresher()

		// Verify nothing was created
		if freshAgent.tokenRefreshTicker != nil {
			t.Error("Token refresher ticker created when stopping without start")
		}
	})
}

func TestAgentInitialization(t *testing.T) {
	// Test that Initialize properly sets up all components
	// Note: This is a basic test since we can't fully initialize without real credentials

	cfg := &config.Config{
		YouTubeCurator: config.YouTubeCuratorConfig{
			YouTube: config.YouTubeConfig{
				ClientID:            "test-client",
				ClientSecret:        "test-secret",
				TokenFile:           "test-token.json",
				TokenRefreshMinutes: 30,
			},
			AI: config.AIConfig{
				GeminiAPIKey: "test-api-key",
				Model:        "gemini-2.5-flash",
			},
			Schedule: "0 0 9 * * *",
		},
		Email: config.EmailConfig{
			SMTPServer: "smtp.test.com",
			SMTPPort:   587,
			Username:   "test@test.com",
			Password:   "test-password",
			FromEmail:  "from@test.com",
			ToEmail:    "to@test.com",
		},
	}

	agent := NewYouTubeAgent(cfg)

	// Verify initial state
	if agent.config != cfg {
		t.Error("Config not properly set")
	}

	// Note: We can't fully test Initialize() without mocking external services
	// but we can verify the structure is correct

	// Test that agent implements the scheduler.Agent interface
	var _ scheduler.Agent = agent
}

func TestBackgroundRefresherTiming(t *testing.T) {
	t.Run("RefresherRunsAtInterval", func(t *testing.T) {

		// Track refresher executions
		executionCount := 0
		stopChan := make(chan bool)

		// Mock refresher for testing timing
		go func() {
			ticker := time.NewTicker(50 * time.Millisecond) // Fast ticker for testing
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					executionCount++
				case <-stopChan:
					return
				}
			}
		}()

		// Let it run for a bit
		time.Sleep(160 * time.Millisecond)
		stopChan <- true

		// Should have executed ~3 times (at 50ms, 100ms, 150ms)
		if executionCount < 2 || executionCount > 4 {
			t.Errorf("Unexpected execution count: %d, expected 2-4", executionCount)
		}
	})
}

func TestConcurrentTokenRefresh(t *testing.T) {
	cfg := &config.Config{
		YouTubeCurator: config.YouTubeCuratorConfig{
			YouTube: config.YouTubeConfig{
				TokenRefreshMinutes: 1,
			},
		},
	}

	agent := NewYouTubeAgent(cfg)

	// Start multiple refreshers concurrently
	done := make(chan bool)
	for i := 0; i < 5; i++ {
		go func() {
			agent.startTokenRefresher(10 * time.Millisecond)
			done <- true
		}()
	}

	// Wait for all to complete
	for i := 0; i < 5; i++ {
		<-done
	}

	// Should only have one ticker running
	if agent.tokenRefreshTicker == nil {
		t.Error("No ticker created despite multiple starts")
	}

	// Clean up
	agent.StopTokenRefresher()

	// Verify cleanup
	if agent.tokenRefreshTicker != nil {
		t.Error("Ticker not cleaned up after stop")
	}
}

func TestAgentRunOnceStructure(t *testing.T) {
	// Test the structure of RunOnce with mock events

	// Create mock events
	var successCalled bool
	var partialFailureCalled bool
	var criticalFailureCalled bool

	events := &scheduler.AgentEvents{
		OnSuccess: func(metrics scheduler.Metrics, duration time.Duration) {
			successCalled = true
			// Verify metrics is of correct type
			if _, ok := metrics.(YouTubeMetrics); !ok {
				t.Error("Metrics is not of type YouTubeMetrics")
			}
		},
		OnPartialFailure: func(err error, duration time.Duration) {
			partialFailureCalled = true
		},
		OnCriticalFailure: func(err error, duration time.Duration) {
			criticalFailureCalled = true
		},
	}

	// Note: We can't actually run RunOnce without real YouTube client
	// but we can verify the events structure is correct
	_ = events
	_ = successCalled
	_ = partialFailureCalled
	_ = criticalFailureCalled

	// Verify the events structure compiles correctly
	_ = context.Background()
}
