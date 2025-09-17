package monitoring

import (
	"fmt"
	"log"
	"net/http"
)

type HealthServer struct {
	monitor *Monitor
	port    string
}

func NewHealthServer(monitor *Monitor, port string) *HealthServer {
	if port == "" {
		port = "8080"
	}
	return &HealthServer{
		monitor: monitor,
		port:    port,
	}
}

func (h *HealthServer) Start() {
	http.HandleFunc("/health", h.healthHandler)
	http.HandleFunc("/status", h.statusHandler)

	log.Printf("Health check server starting on port %s", h.port)
	go func() {
		if err := http.ListenAndServe(":"+h.port, nil); err != nil {
			log.Printf("Health server error: %v", err)
		}
	}()
}

func (h *HealthServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	if h.monitor.IsHealthy() {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK - %s", h.monitor.GetStatusSummary())
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, "Service unhealthy - %s", h.monitor.GetStatusSummary())
	}
}

func (h *HealthServer) statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "%s", h.monitor.GetStatusSummary())
}
