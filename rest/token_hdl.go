package rest

import (
	"net/http"
	"time"

	"github.com/Gthulhu/api/util"
)

// TokenResponse represents the response structure for JWT token generation
type TokenResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	Token     string `json:"token,omitempty"`
}

// TokenRequest represents the request structure for JWT token generation
type TokenRequest struct {
	PublicKey string `json:"public_key"` // PEM encoded public key
}

// GenTokenHandler handles JWT token generation upon public key verification
func (h Handler) GenTokenHandler(w http.ResponseWriter, r *http.Request) {
	var req TokenRequest
	err := h.JSONBind(r, &req)
	if err != nil {
		h.ErrorResponse(w, http.StatusBadRequest, "Invalid JSON format: "+err.Error())
		return
	}
	token, err := h.Service.VerifyAndGenerateToken(r.Context(), req.PublicKey)
	if err != nil {
		h.ErrorResponse(w, http.StatusUnauthorized, "Public key verification failed: "+err.Error())
		return
	}

	util.GetLogger().Debug("Generated JWT token for client")

	resp := TokenResponse{
		Success:   true,
		Message:   "Token generated successfully",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Token:     token,
	}
	h.JSONResponse(w, http.StatusOK, resp)
}
