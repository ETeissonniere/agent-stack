package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"agent-stack/shared/config"
	"agent-stack/shared/scheduler"
	"agent-stack/agents/youtube-curator"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	ctx := context.Background()
	
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
	s.Start(ctx)
}
