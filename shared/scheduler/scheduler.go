package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"

	"agent-stack/shared/config"
	"agent-stack/shared/monitoring"

	"github.com/robfig/cron/v3"
)

// Agent defines the interface that all agents must implement
type Agent interface {
	Name() string
	RunOnce(ctx context.Context) error
	Initialize() error
}

// Scheduler manages the execution of agents on a schedule
type Scheduler struct {
	config  *config.Config
	monitor *monitoring.Monitor
	agent   Agent
	cron    *cron.Cron
}

func New(cfg *config.Config, agent Agent) *Scheduler {
	return &Scheduler{
		config:  cfg,
		monitor: monitoring.NewMonitor(),
		agent:   agent,
		cron:    cron.New(cron.WithSeconds()),
	}
}

func (s *Scheduler) Start(ctx context.Context) error {
	if err := s.agent.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	// Start health check server
	healthServer := monitoring.NewHealthServer(s.monitor, "8080")
	healthServer.Start()

	_, err := s.cron.AddFunc(s.config.Schedule, func() {
		if err := s.RunOnce(ctx); err != nil {
			log.Printf("Error running scheduled job for %s: %v", s.agent.Name(), err)
		}
	})
	if err != nil {
		return fmt.Errorf("failed to add cron job: %w", err)
	}

	log.Printf("Scheduler started for %s with schedule: %s", s.agent.Name(), s.config.Schedule)
	s.cron.Start()

	// Keep the scheduler running
	select {
	case <-ctx.Done():
		log.Printf("Scheduler stopped for %s", s.agent.Name())
		s.cron.Stop()
		return ctx.Err()
	}
}

func (s *Scheduler) RunOnce(ctx context.Context) error {
	startTime := time.Now()
	agentName := s.agent.Name()
	
	log.Printf("Starting %s run...", agentName)
	
	if err := s.agent.RunOnce(ctx); err != nil {
		duration := time.Since(startTime)
		s.monitor.RecordFailure(fmt.Errorf("%s failed: %w", agentName, err), duration)
		return fmt.Errorf("%s run failed: %w", agentName, err)
	}

	// Record successful completion - agents can provide their own metrics via monitor
	duration := time.Since(startTime)
	s.monitor.RecordSuccess(0, 0, 0, duration) // Generic success - agents can override
	
	log.Printf("%s run completed successfully in %v", agentName, duration)
	return nil
}