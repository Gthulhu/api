package rest

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Gthulhu/api/util"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/xid"
)

// getJwtAuthMiddleware returns a middleware that validates JWT tokens
func getJwtAuthMiddleware(rasKey *rsa.PrivateKey) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth for OPTIONS requests, health check, root endpoint, token endpoint, and static files
			if r.Method == "OPTIONS" ||
				r.URL.Path == "/health" ||
				r.URL.Path == "/" ||
				r.URL.Path == "/api/v1/auth/token" ||
				strings.HasPrefix(r.URL.Path, "/static/") {
				next.ServeHTTP(w, r)
				return
			}

			// Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				if err := json.NewEncoder(w).Encode(ErrorResponse{
					Success: false,
					Error:   "Authorization header is required",
				}); err != nil {
					log.Printf("Error encoding response: %v", err)
				}
				return
			}

			// Check Bearer token format
			const bearerSchema = "Bearer "
			if !strings.HasPrefix(authHeader, bearerSchema) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				if err := json.NewEncoder(w).Encode(ErrorResponse{
					Success: false,
					Error:   "Authorization header must start with 'Bearer '",
				}); err != nil {
					log.Printf("Error encoding response: %v", err)
				}
				return
			}

			tokenString := authHeader[len(bearerSchema):]

			// Validate JWT token
			claims, err := validateJWT(rasKey, tokenString)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				if err := json.NewEncoder(w).Encode(ErrorResponse{
					Success: false,
					Error:   "Invalid or expired token: " + err.Error(),
				}); err != nil {
					log.Printf("Error encoding response: %v", err)
				}
				return
			}

			log.Printf("Authenticated request from client: %s", claims.ClientID)
			next.ServeHTTP(w, r)
		})
	}
}

// Claims represents JWT token claims
type Claims struct {
	ClientID string `json:"client_id"`
	jwt.RegisteredClaims
}

// validateJWT validates a JWT token and returns the claims
func validateJWT(rasKey *rsa.PrivateKey, tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return &rasKey.PublicKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// CORS middleware
func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		// Allow Authorization for JWT, and X-Request-ID for tracing
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Logging middleware
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		logger := util.GetLogger()

		ctx := r.Context()
		logger = logger.With(slog.String("method", r.Method), slog.String("request_uri", r.RequestURI), slog.String("remote_addr", r.RemoteAddr))

		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = xid.New().String()
		}
		logger = logger.With(slog.String("request_id", reqID))
		r = r.WithContext(util.AddLoggerToCtx(ctx, logger))
		logger.Debug("access log")
		log.Printf("%s %s %s", r.Method, r.RequestURI, r.RemoteAddr)

		rec := &statusRecorder{ResponseWriter: w}

		next.ServeHTTP(rec, r)

		if rec.Status >= 500 {
			logger.Warn("response log",
				slog.Int("status", rec.Status),
				slog.Int("response_bytes", rec.Bytes),
				slog.Duration("duration", time.Since(start)),
			)
		} else if rec.Status >= 400 {
			logger.Error("response log",
				slog.Int("status", rec.Status),
				slog.Int("response_bytes", rec.Bytes),
				slog.Duration("duration", time.Since(start)),
			)
		} else {
			logger.Info("response log",
				slog.Int("status", rec.Status),
				slog.Int("response_bytes", rec.Bytes),
				slog.Duration("duration", time.Since(start)),
			)
		}
	})
}

type statusRecorder struct {
	http.ResponseWriter
	Status int
	Bytes  int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.Status = code
	rec.ResponseWriter.WriteHeader(code)
}

func (rec *statusRecorder) Write(b []byte) (int, error) {
	// 如果沒有明確呼叫 WriteHeader 就是 200
	if rec.Status == 0 {
		rec.Status = http.StatusOK
	}
	n, err := rec.ResponseWriter.Write(b)
	rec.Bytes += n
	return n, err
}
