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

func (m *Monitor) RecordSuccess(summary string, duration time.Duration) {
	m.lastRunSuccess = true
	m.lastRunTime = time.Now()
	
	log.Printf("✅ Run completed successfully - %s (took %v)", summary, duration)
}

func (m *Monitor) RecordPartialFailure(err error, duration time.Duration) {
	// Don't change health status for partial failures
	log.Printf("⚠️  PARTIAL FAILURE: %s (Duration: %v)", err.Error(), duration)
}

func (m *Monitor) RecordCriticalFailure(err error, duration time.Duration) {
	m.lastRunSuccess = false
	m.lastRunTime = time.Now()
	
	log.Printf("🚨 CRITICAL FAILURE: %s (Duration: %v)", err.Error(), duration)
	log.Printf("Failure occurred at: %s", time.Now().Format("2006-01-02 15:04:05"))
}

func (m *Monitor) IsHealthy() bool {
	if m.lastRunTime.IsZero() {
		return true // No runs yet, assume healthy
	}
	
	// Simple and reliable: healthy if last run was successful
	return m.lastRunSuccess
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