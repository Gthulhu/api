package rest

import (
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"time"

	"github.com/Gthulhu/api/config"
	"github.com/Gthulhu/api/domain"
	"github.com/Gthulhu/api/util"
)

// ErrorResponse represents error response structure
type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

// SuccessResponse represents the success response structure
type SuccessResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

type Params struct {
	Service       domain.Service
	JWTPrivateKey *rsa.PrivateKey
	Config        *config.Config
}

func NewHandler(params Params) *Handler {
	return &Handler{
		Service:       params.Service,
		jwtPrivateKey: params.JWTPrivateKey,
		Config:        params.Config,
	}
}

type Handler struct {
	domain.Service
	Config        *config.Config
	jwtPrivateKey *rsa.PrivateKey
}

func (h *Handler) JSONResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		util.GetLogger().Error("Failed to encode JSON response", util.LogErrAttr(err))
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

func (h *Handler) ErrorResponse(w http.ResponseWriter, status int, errMsg string) {
	resp := ErrorResponse{
		Success: false,
		Error:   errMsg,
	}
	h.JSONResponse(w, status, resp)
}

func (h *Handler) SuccessResponse(w http.ResponseWriter, message string) {
	resp := SuccessResponse{
		Success:   true,
		Message:   message,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	h.JSONResponse(w, http.StatusOK, resp)
}

func (h *Handler) Version(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"message":   "BSS Metrics API Server",
		"version":   "1.0.0",
		"endpoints": "/api/v1/auth/token (POST), /api/v1/metrics (POST), /api/v1/pods/pids (GET), /api/v1/scheduling/strategies (GET, POST), /health (GET), /static/ (Frontend)",
	}
	h.JSONResponse(w, http.StatusOK, response)
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"service":   "BSS Metrics API Server",
	}
	h.JSONResponse(w, http.StatusOK, response)
}
