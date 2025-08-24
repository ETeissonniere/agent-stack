package monitoring

import (
	"fmt"
	"log"
	"time"
)

type Monitor struct {
	lastRunSuccess bool
	lastRunTime    time.Time
}

func NewMonitor() *Monitor {
	return &Monitor{}
}

func (m *Monitor) RecordSuccess(videosFound, videosAnalyzed, videosSelected int, duration time.Duration) {
	m.lastRunSuccess = true
	m.lastRunTime = time.Now()
	
	log.Printf("✅ Run completed successfully - found %d videos, analyzed %d, selected %d (took %v)", 
		videosFound, videosAnalyzed, videosSelected, duration)
}

func (m *Monitor) RecordFailure(err error, duration time.Duration) {
	m.lastRunSuccess = false
	m.lastRunTime = time.Now()
	
	log.Printf("🚨 CRON JOB FAILED: %s (Duration: %v)", err.Error(), duration)
	log.Printf("Failure occurred at: %s", time.Now().Format("2006-01-02 15:04:05"))
}

func (m *Monitor) IsHealthy() bool {
	if m.lastRunTime.IsZero() {
		return true // No runs yet, assume healthy
	}
	
	// Consider unhealthy if more than 26 hours since last run
	return time.Since(m.lastRunTime) <= 26*time.Hour
}

func (m *Monitor) GetStatusSummary() string {
	if m.lastRunTime.IsZero() {
		return "No runs yet"
	}
	
	if m.lastRunSuccess {
		return fmt.Sprintf("✅ Last run: %s", m.lastRunTime.Format("Jan 2 15:04"))
	} else {
		return fmt.Sprintf("❌ Last run failed: %s", m.lastRunTime.Format("Jan 2 15:04"))
	}
}