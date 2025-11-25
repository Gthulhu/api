package rest

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func (h *Handler) SetupRoutes(engine *echo.Echo) {
	engine.GET("/health", h.echoHandler(h.HealthCheck))
	engine.GET("/version", h.echoHandler(h.Version))

	// v1 routes
	{
		// users & auth routes
		engine.POST("/api/v1/users", h.echoHandler(h.CreateUser), echo.WrapMiddleware(h.AuthMiddleware))
		engine.DELETE("/api/v1/users", h.echoHandler(h.DeleteUser), echo.WrapMiddleware(h.AuthMiddleware))
		engine.PUT("/api/v1/users", h.echoHandler(h.UpdateUser), echo.WrapMiddleware(h.AuthMiddleware))
		engine.PUT("/api/v1/users/password", h.echoHandler(h.ChangePassword), echo.WrapMiddleware(h.AuthMiddleware))
		engine.POST("/api/v1/auth/login", h.echoHandler(h.Login), echo.WrapMiddleware(h.AuthMiddleware))
	}

}

func (h *Handler) echoHandler(handlerFunc func(w http.ResponseWriter, r *http.Request)) echo.HandlerFunc {
	return echo.WrapHandler(http.HandlerFunc(handlerFunc))
}

func (h *Handler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Authentication logic here (e.g., check JWT token)

		// If authenticated, proceed to the next handler
		next.ServeHTTP(w, r)
	})
}
