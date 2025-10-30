package rest

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/Gthulhu/api/domain"
	"github.com/Gthulhu/api/service"
	"github.com/Gthulhu/api/util"
)

// SaveMetricsRequest represents the request structure for saving BSS metrics
type SaveMetricsRequest struct {
	Usersched_last_run_at uint64 `json:"usersched_last_run_at"` // The PID of the userspace scheduler
	Nr_queued             uint64 `json:"nr_queued"`             // Number of tasks queued in the userspace scheduler
	Nr_scheduled          uint64 `json:"nr_scheduled"`          // Number of tasks scheduled by the userspace scheduler
	Nr_running            uint64 `json:"nr_running"`            // Number of tasks currently running in the userspace scheduler
	Nr_online_cpus        uint64 `json:"nr_online_cpus"`        // Number of online CPUs in the system
	Nr_user_dispatches    uint64 `json:"nr_user_dispatches"`    // Number of user-space dispatches
	Nr_kernel_dispatches  uint64 `json:"nr_kernel_dispatches"`  // Number of kernel-space dispatches
	Nr_cancel_dispatches  uint64 `json:"nr_cancel_dispatches"`  // Number of cancelled dispatches
	Nr_bounce_dispatches  uint64 `json:"nr_bounce_dispatches"`  // Number of bounce dispatches
	Nr_failed_dispatches  uint64 `json:"nr_failed_dispatches"`  // Number of failed dispatches
	Nr_sched_congested    uint64 `json:"nr_sched_congested"`    // Number of times the scheduler was congested
}

func (req *SaveMetricsRequest) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Uint64("usersched_last_run_at", req.Usersched_last_run_at),
		slog.Uint64("nr_queued", req.Nr_queued),
		slog.Uint64("nr_scheduled", req.Nr_scheduled),
		slog.Uint64("nr_running", req.Nr_running),
		slog.Uint64("nr_online_cpus", req.Nr_online_cpus),
		slog.Uint64("nr_user_dispatches", req.Nr_user_dispatches),
		slog.Uint64("nr_kernel_dispatches", req.Nr_kernel_dispatches),
		slog.Uint64("nr_cancel_dispatches", req.Nr_cancel_dispatches),
		slog.Uint64("nr_bounce_dispatches", req.Nr_bounce_dispatches),
		slog.Uint64("nr_failed_dispatches", req.Nr_failed_dispatches),
		slog.Uint64("nr_sched_congested", req.Nr_sched_congested),
	)
}

// SaveMetricsHandler handles saving BSS metrics data
func (h *Handler) SaveMetricsHandler(w http.ResponseWriter, r *http.Request) {
	var req SaveMetricsRequest
	err := h.JSONBind(r, &req)
	if err != nil {
		h.ErrorResponse(w, http.StatusBadRequest, "Invalid JSON format: "+err.Error())
		return
	}

	bssData := domain.BssData{
		Usersched_last_run_at: req.Usersched_last_run_at,
		Nr_queued:             req.Nr_queued,
		Nr_scheduled:          req.Nr_scheduled,
		Nr_running:            req.Nr_running,
		Nr_online_cpus:        req.Nr_online_cpus,
		Nr_user_dispatches:    req.Nr_user_dispatches,
		Nr_kernel_dispatches:  req.Nr_kernel_dispatches,
		Nr_cancel_dispatches:  req.Nr_cancel_dispatches,
		Nr_bounce_dispatches:  req.Nr_bounce_dispatches,
		Nr_failed_dispatches:  req.Nr_failed_dispatches,
		Nr_sched_congested:    req.Nr_sched_congested,
		UpdatedTime:           time.Now(),
	}

	err = h.Service.SaveBSSMetrics(r.Context(), &bssData)
	if err != nil {
		h.ErrorResponse(w, http.StatusInternalServerError, "Failed to save metrics: "+err.Error())
		return
	}

	util.GetLogger().Info("Saved BSS metrics", slog.Any("metrics", req))
	h.SuccessResponse(w, "Metrics saved successfully")
}

// GetMetricsResponse represents the response structure for getting current metrics
type GetMetricsResponse struct {
	Success          bool            `json:"success"`
	Message          string          `json:"message"`
	Timestamp        string          `json:"timestamp"`
	Data             *domain.BssData `json:"data,omitempty"`
	MetricsTimestamp string          `json:"metrics_timestamp,omitempty"`
}

// GetMetricsHandler handles retrieving the latest BSS metrics data
func (h *Handler) GetMetricsHandler(w http.ResponseWriter, r *http.Request) {
	bssData, err := h.Service.GetBSSMetrics(r.Context())
	if err != nil {
		if errors.Is(err, service.ErrNoBssData) {
			response := GetMetricsResponse{
				Success:   false,
				Message:   "No metrics data available yet",
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			}
			h.JSONResponse(w, http.StatusOK, response)
			return
		}

		h.ErrorResponse(w, http.StatusInternalServerError, "Failed to get metrics: "+err.Error())
		return
	}

	util.GetLogger().Info("Retrieved BSS metrics")

	resp := GetMetricsResponse{
		Success:          true,
		Message:          "Metrics retrieved successfully",
		Timestamp:        time.Now().UTC().Format(time.RFC3339),
		Data:             bssData,
		MetricsTimestamp: bssData.UpdatedTime.UTC().Format(time.RFC3339),
	}
	h.JSONResponse(w, http.StatusOK, resp)
}
