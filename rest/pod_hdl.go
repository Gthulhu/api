package rest

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/Gthulhu/api/domain"
	"github.com/Gthulhu/api/util"
)

// PodPidResponse represents the response structure for pod-pid mapping
type GetPodPidResponse struct {
	Success   bool              `json:"success"`
	Message   string            `json:"message"`
	Timestamp string            `json:"timestamp"`
	Pods      []*domain.PodInfo `json:"pods"`
}

func (h *Handler) GetPodPidHandler(w http.ResponseWriter, r *http.Request) {
	podInfos, err := h.Service.GetAllPodInfos(r.Context())
	if err != nil {
		h.ErrorResponse(w, http.StatusInternalServerError, "Failed to get pod infos: "+err.Error())
		return
	}

	util.GetLogger().Debug("Retrieved pod-pid mappings", slog.Int("pod_count", len(podInfos)))

	resp := GetPodPidResponse{
		Success:   true,
		Message:   "Pod-PID mappings retrieved successfully",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Pods:      podInfos,
	}
	h.JSONResponse(w, http.StatusOK, resp)
}
