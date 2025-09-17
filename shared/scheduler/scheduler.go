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

// Metrics defines the common interface for agent metrics
type Metrics interface {
	// GetSummary returns a human-readable summary of the run
	GetSummary() string
}

// AgentEvents provides callbacks for monitoring agent execution
type AgentEvents struct {
	OnSuccess         func(metrics Metrics, duration time.Duration)
	OnPartialFailure  func(err error, duration time.Duration)
	OnCriticalFailure func(err error, duration time.Duration)
}

// Agent defines the interface that all agents must implement
type Agent interface {
	Name() string
	RunOnce(ctx context.Context, events *AgentEvents) error
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
	m := monitoring.NewMonitor()

	return &Scheduler{
		config:  cfg,
		monitor: m,
		agent:   agent,
		// Prevent overlapping runs
		cron: cron.New(cron.WithSeconds(), cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger))),
	}
}

func (s *Scheduler) Start(ctx context.Context) error {
	if err := s.agent.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	// Start health check server (configurable via config, defaults to 8080)
	healthServer := monitoring.NewHealthServer(s.monitor, fmt.Sprintf("%d", s.config.Monitoring.HealthPort))
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

	// Keep the scheduler running indefinitely until context is cancelled
	<-ctx.Done()
	log.Printf("Scheduler stopped for %s", s.agent.Name())
	s.cron.Stop()
	return ctx.Err()
}

func (s *Scheduler) RunOnce(ctx context.Context) error {
	startTime := time.Now()
	agentName := s.agent.Name()

	log.Printf("Starting %s run...", agentName)

	// Create event handlers for monitoring
	events := &AgentEvents{
		OnSuccess: func(metrics Metrics, duration time.Duration) {
			s.monitor.RecordSuccess(metrics.GetSummary(), duration)
		},
		OnPartialFailure: func(err error, duration time.Duration) {
			s.monitor.RecordPartialFailure(fmt.Errorf("%s partial failure: %w", agentName, err), duration)
		},
		OnCriticalFailure: func(err error, duration time.Duration) {
			s.monitor.RecordCriticalFailure(fmt.Errorf("%s critical failure: %w", agentName, err), duration)
		},
	}

	if err := s.agent.RunOnce(ctx, events); err != nil {
		duration := time.Since(startTime)
		s.monitor.RecordCriticalFailure(fmt.Errorf("%s failed: %w", agentName, err), duration)
		return fmt.Errorf("%s run failed: %w", agentName, err)
	}

	return nil
}
