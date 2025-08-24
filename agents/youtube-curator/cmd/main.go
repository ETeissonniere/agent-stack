package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"agent-stack/shared/config"
	"agent-stack/shared/scheduler"
	"agent-stack/agents/youtube-curator"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create context that responds to signals
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	
	// Create YouTube agent and scheduler
	agent := youtubecurator.NewYouTubeAgent(cfg)
	s := scheduler.New(cfg, agent)

	if len(os.Args) > 1 && os.Args[1] == "--once" {
		fmt.Println("Running once...")
		if err := agent.Initialize(); err != nil {
			log.Fatalf("Failed to initialize agent: %v", err)
		}
		
		if err := s.RunOnce(ctx); err != nil {
			log.Fatalf("Failed to run: %v", err)
		}
		return
	}

	fmt.Println("Starting scheduler...")
	if err := s.Start(ctx); err != nil {
		log.Fatalf("Scheduler failed: %v", err)
	}
}
