package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"time"
)

// HealthStatus represents the health check response
type HealthStatus struct {
	Status    string            `json:"status"`
	Timestamp string            `json:"timestamp"`
	Version   string            `json:"version,omitempty"`
	Checks    map[string]Check  `json:"checks,omitempty"`
	System    *SystemInfo       `json:"system,omitempty"`
}

// Check represents a single health check result
type Check struct {
	Status   string `json:"status"`
	Message  string `json:"message,omitempty"`
	Duration string `json:"duration,omitempty"`
}

// SystemInfo contains system information
type SystemInfo struct {
	GoVersion    string `json:"go_version"`
	NumGoroutine int    `json:"num_goroutine"`
	NumCPU       int    `json:"num_cpu"`
	MemAlloc     uint64 `json:"mem_alloc_mb"`
}

const (
	StatusHealthy   = "healthy"
	StatusUnhealthy = "unhealthy"
	StatusDegraded  = "degraded"
)

// Health returns basic health status (for load balancer)
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	status := HealthStatus{
		Status:    StatusHealthy,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

// Ready performs full readiness check including dependencies
func (h *Handlers) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	checks := make(map[string]Check)
	overallStatus := StatusHealthy

	// Check database
	dbCheck := h.checkDatabase(ctx)
	checks["database"] = dbCheck
	if dbCheck.Status != StatusHealthy {
		overallStatus = StatusUnhealthy
	}

	// Check Redis
	redisCheck := h.checkRedis(ctx)
	checks["redis"] = redisCheck
	if redisCheck.Status != StatusHealthy {
		if overallStatus == StatusHealthy {
			overallStatus = StatusDegraded
		}
	}

	// Check queue
	queueCheck := h.checkQueue()
	checks["queue"] = queueCheck

	// System info
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	sysInfo := &SystemInfo{
		GoVersion:    runtime.Version(),
		NumGoroutine: runtime.NumGoroutine(),
		NumCPU:       runtime.NumCPU(),
		MemAlloc:     memStats.Alloc / 1024 / 1024, // Convert to MB
	}

	status := HealthStatus{
		Status:    overallStatus,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    checks,
		System:    sysInfo,
	}

	w.Header().Set("Content-Type", "application/json")
	if overallStatus == StatusUnhealthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	json.NewEncoder(w).Encode(status)
}

// checkDatabase verifies database connectivity
func (h *Handlers) checkDatabase(ctx context.Context) Check {
	start := time.Now()

	err := h.Repo.DB().Pool().Ping(ctx)
	duration := time.Since(start)

	if err != nil {
		return Check{
			Status:   StatusUnhealthy,
			Message:  err.Error(),
			Duration: duration.String(),
		}
	}

	return Check{
		Status:   StatusHealthy,
		Message:  "connection successful",
		Duration: duration.String(),
	}
}

// checkRedis verifies Redis connectivity
func (h *Handlers) checkRedis(ctx context.Context) Check {
	start := time.Now()

	// Use the Client() method we added
	err := h.Redis.Client().Ping(ctx).Err()
	duration := time.Since(start)

	if err != nil {
		return Check{
			Status:   StatusUnhealthy,
			Message:  err.Error(),
			Duration: duration.String(),
		}
	}

	return Check{
		Status:   StatusHealthy,
		Message:  "connection successful",
		Duration: duration.String(),
	}
}

// checkQueue returns queue status
func (h *Handlers) checkQueue() Check {
	queueLen := h.Q.Len()

	status := StatusHealthy
	message := "queue operational"

	// Warn if queue is getting full (arbitrary threshold)
	if queueLen > 500 {
		status = StatusDegraded
		message = "queue backlog detected"
	}

	return Check{
		Status:  status,
		Message: fmt.Sprintf("%s (pending: %d)", message, queueLen),
	}
}

