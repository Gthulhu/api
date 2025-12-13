package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/Gthulhu/api/decisionmaker/domain"
	"github.com/Gthulhu/api/decisionmaker/service"
	"github.com/Gthulhu/api/manager/errs"
	"github.com/Gthulhu/api/pkg/logger"
	"github.com/Gthulhu/api/pkg/middleware"
	"github.com/labstack/echo/v4"
	"go.uber.org/fx"
)

// ErrorResponse represents error response structure
type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

// EmptyResponse is used for endpoints that return no data payload.
type EmptyResponse struct{}

// VersionResponse describes the version endpoint payload.
type VersionResponse struct {
	Message   string `json:"message"`
	Version   string `json:"version"`
	Endpoints string `json:"endpoints"`
}

// HealthResponse describes the health check payload.
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Service   string `json:"service"`
}

func NewSuccessResponse[T any](data *T) SuccessResponse[T] {
	return SuccessResponse[T]{
		Success:   true,
		Data:      data,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// SuccessResponse represents the success response structure
type SuccessResponse[T any] struct {
	Success   bool   `json:"success"`
	Data      *T     `json:"data,omitempty"`
	Timestamp string `json:"timestamp"`
}

type Params struct {
	fx.In
	Service service.Service
}

func NewHandler(params Params) (*Handler, error) {
	return &Handler{
		Service: params.Service,
	}, nil
}

type Handler struct {
	Service service.Service
}

func (h *Handler) JSONResponse(ctx context.Context, w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		logger.Logger(ctx).Error().Err(err).Msg("Failed to encode JSON response")
		http.Error(w, "Failed to encode JSON response", http.StatusInternalServerError)
	}
}

func (h *Handler) JSONBind(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(dst)
	if err != nil {
		return err
	}
	return nil
}

func (h *Handler) HandleError(ctx context.Context, w http.ResponseWriter, err error) {
	httpErr, ok := errs.IsHTTPStatusError(err)
	if ok {
		h.ErrorResponse(ctx, w, httpErr.StatusCode, httpErr.Message, httpErr.OriginalErr)
		return
	}
	h.ErrorResponse(ctx, w, http.StatusInternalServerError, "Internal Server Error", err)
}

func (h *Handler) ErrorResponse(ctx context.Context, w http.ResponseWriter, status int, errMsg string, err error) {
	if err != nil {
		if status >= 500 {
			logger.Logger(ctx).Error().Err(err).Msg(errMsg)
		} else {
			logger.Logger(ctx).Warn().Err(err).Msg(errMsg)
		}
	}
	resp := ErrorResponse{
		Success: false,
		Error:   errMsg,
	}
	h.JSONResponse(ctx, w, status, resp)
}

// Version godoc
// @Summary Get service version
// @Description Returns service version and exposed endpoints.
// @Tags System
// @Produce json
// @Success 200 {object} VersionResponse
// @Router /version [get]
func (h *Handler) Version(w http.ResponseWriter, r *http.Request) {
	response := VersionResponse{
		Message:   "BSS Metrics API Server",
		Version:   "1.0.0",
		Endpoints: "/api/v1/auth/token (POST), /api/v1/metrics (POST), /api/v1/pods/pids (GET), /api/v1/scheduling/strategies (GET, POST), /health (GET), /static/ (Frontend)",
	}
	h.JSONResponse(r.Context(), w, http.StatusOK, response)
}

// HealthCheck godoc
// @Summary Health check
// @Description Basic health check for readiness probes.
// @Tags System
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /health [get]
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Service:   "BSS Metrics API Server",
	}
	h.JSONResponse(r.Context(), w, http.StatusOK, response)
}

type HandleIntentsRequest struct {
	Intents []Intent `json:"intents"`
}

type Intent struct {
	PodName       string            `json:"podName,omitempty"`
	PodID         string            `json:"podID,omitempty"`
	NodeID        string            `json:"nodeID,omitempty"`
	K8sNamespace  string            `json:"k8sNamespace,omitempty"`
	CommandRegex  string            `json:"commandRegex,omitempty"`
	Priority      int               `json:"priority,omitempty"`
	ExecutionTime int64             `json:"executionTime,omitempty"`
	PodLabels     map[string]string `json:"podLabels,omitempty"`
}

func (h *Handler) HandleIntents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req HandleIntentsRequest
	err := h.JSONBind(r, &req)
	if err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Invalid request payload", err)
		return
	}
	intents := make([]*domain.Intent, 0, len(req.Intents))
	for _, intent := range req.Intents {
		intents = append(intents, &domain.Intent{
			PodName:       intent.PodName,
			PodID:         intent.PodID,
			NodeID:        intent.NodeID,
			K8sNamespace:  intent.K8sNamespace,
			CommandRegex:  intent.CommandRegex,
			Priority:      intent.Priority,
			ExecutionTime: intent.ExecutionTime,
			PodLabels:     intent.PodLabels,
		})
	}
	// TODO: forward intents to the ebpf user space agent
	h.Service.ProcessIntents(r.Context(), intents)
	h.JSONResponse(ctx, w, http.StatusOK, NewSuccessResponse[EmptyResponse](nil))
}

func (h *Handler) SetupRoutes(engine *echo.Echo) {
	engine.GET("/health", h.echoHandler(h.HealthCheck))
	engine.GET("/version", h.echoHandler(h.Version))
	// docs.SwaggerInfo.BasePath = "/"
	// engine.GET("/swagger/*", echoSwagger.WrapHandler)

	api := engine.Group("/api", echo.WrapMiddleware(middleware.LoggerMiddleware))
	// v1 routes
	{
		apiV1 := api.Group("/v1")
		// auth routes
		apiV1.POST("/intents", h.echoHandler(h.HandleIntents))
	}

}

func (h *Handler) echoHandler(handlerFunc func(w http.ResponseWriter, r *http.Request)) echo.HandlerFunc {
	return echo.WrapHandler(http.HandlerFunc(handlerFunc))
}
