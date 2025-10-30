package rest

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Gthulhu/api/domain"
)

// GetSchedulingStrategiesResponse represents the response structure for scheduling strategies
type GetSchedulingStrategiesResponse struct {
	Success    bool                         `json:"success"`
	Message    string                       `json:"message"`
	Timestamp  string                       `json:"timestamp"`
	Scheduling []*domain.SchedulingStrategy `json:"scheduling"`
}

// GetSchedulingStrategiesHandler handles the retrieval of current scheduling strategies
func (h *Handler) GetSchedulingStrategiesHandler(w http.ResponseWriter, r *http.Request) {
	finalStrategies, fromCache, err := h.Service.FindCurrentUsingSchedulingStrategiesWithPID(r.Context())
	if err != nil {
		h.ErrorResponse(w, http.StatusInternalServerError, "Failed to get scheduling strategies"+err.Error())
	}

	// If not from cache, strategies were recalculated in GetCachedStrategies
	var message string
	if fromCache {
		message = "Scheduling strategies retrieved from cache"
	} else {
		message = "Scheduling strategies recalculated due to pod changes"
	}

	cacheStats := h.Service.GetStrategyCacheStats()
	// Add cache stats as header for debugging
	w.Header().Set("X-Cache-Hit", fmt.Sprintf("%v", fromCache))
	w.Header().Set("X-Cache-Stats", fmt.Sprintf("hits=%d,misses=%d,hit_rate=%v",
		cacheStats["hits"], cacheStats["misses"], cacheStats["hit_rate"]))

	// Send success response with cache info
	response := GetSchedulingStrategiesResponse{
		Success:    true,
		Message:    message,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Scheduling: finalStrategies,
	}
	h.JSONResponse(w, http.StatusOK, response)
}

// StrategyRequest represents the request structure for setting scheduling strategies
type SaveStrategyRequest struct {
	Strategies []*domain.SchedulingStrategy `json:"strategies"`
}

func (h *Handler) SaveSchedulingStrategiesHandler(w http.ResponseWriter, r *http.Request) {
	var req SaveStrategyRequest
	err := h.JSONBind(r, &req)
	if err != nil {
		h.ErrorResponse(w, http.StatusBadRequest, "Invalid request payload: "+err.Error())
		return
	}

	err = h.Service.SaveSchedulingStrategy(r.Context(), req.Strategies)
	if err != nil {
		h.ErrorResponse(w, http.StatusInternalServerError, "Failed to save scheduling strategies: "+err.Error())
		return
	}
	h.SuccessResponse(w, "Scheduling strategies saved successfully")
}
