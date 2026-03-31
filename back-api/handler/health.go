package handler

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"time"
)

// HealthStatus represents the health check response
type HealthStatus struct {
	Status    string           `json:"status"`
	Timestamp string           `json:"timestamp"`
	Version   string           `json:"version,omitempty"`
	Checks    map[string]Check `json:"checks,omitempty"`
	System    *SystemInfo      `json:"system,omitempty"`
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

	queueBacklogThreshold = 500 // warn when queue has more pending jobs
)

// Health returns basic health status (for load balancer)
func (*HealthHandler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, HealthStatus{
		Status:    StatusHealthy,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// Ready performs full readiness check including dependencies
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
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

	// Check storage
	storageCheck := h.checkStorage(ctx)
	checks["storage"] = storageCheck
	if storageCheck.Status != StatusHealthy {
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
		MemAlloc:     memStats.Alloc / 1024 / 1024,
	}

	status := HealthStatus{
		Status:    overallStatus,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    checks,
		System:    sysInfo,
	}

	code := http.StatusOK
	if overallStatus == StatusUnhealthy {
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, status)
}

func (h *HealthHandler) checkDatabase(ctx context.Context) Check {
	start := time.Now()
	err := h.Repo.Ping(ctx)
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

func (h *HealthHandler) checkRedis(ctx context.Context) Check {
	start := time.Now()
	err := h.Sessions.Ping(ctx)
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

func (h *HealthHandler) checkStorage(ctx context.Context) Check {
	start := time.Now()
	_, err := h.Storage.GetPresignedURL(ctx, "healthcheck", 1*time.Minute)
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
		Message:  "storage accessible",
		Duration: duration.String(),
	}
}

func (h *HealthHandler) checkQueue() Check {
	queueLen := h.Queue.Len()

	status := StatusHealthy
	message := "queue operational"

	if queueLen > queueBacklogThreshold {
		status = StatusDegraded
		message = "queue backlog detected"
	}

	return Check{
		Status:  status,
		Message: fmt.Sprintf("%s (pending: %d)", message, queueLen),
	}
}
