package rest

import (
	"net/http"

	"github.com/Gthulhu/api/docs"
	"github.com/Gthulhu/api/manager/domain"
	"github.com/labstack/echo/v4"
	echoSwagger "github.com/swaggo/echo-swagger"
)

func (h *Handler) SetupRoutes(engine *echo.Echo) {
	engine.GET("/health", h.echoHandler(h.HealthCheck))
	engine.GET("/version", h.echoHandler(h.Version))
	docs.SwaggerInfo.BasePath = "/"
	engine.GET("/swagger/*", echoSwagger.WrapHandler)

	api := engine.Group("/api", echo.WrapMiddleware(LoggerMiddleware))
	// v1 routes
	{
		apiV1 := api.Group("/v1")
		// auth routes
		apiV1.POST("/auth/login", h.echoHandler(h.Login))

		// users  routes
		apiV1.POST("/users", h.echoHandler(h.CreateUser), echo.WrapMiddleware(h.GetAuthMiddleware(domain.CreateUser)))
		apiV1.PUT("/users/password", h.echoHandler(h.ResetPassword), echo.WrapMiddleware(h.GetAuthMiddleware(domain.ResetUserPassword)))
		apiV1.PUT("/users/permissions", h.echoHandler(h.UpdateUserPermissions), echo.WrapMiddleware(h.GetAuthMiddleware(domain.ChangeUserPermission)))
		apiV1.GET("/users", h.echoHandler(h.ListUsers), echo.WrapMiddleware(h.GetAuthMiddleware(domain.UserRead)))
		apiV1.PUT("/users/self/password", h.echoHandler(h.ChangePassword), echo.WrapMiddleware(h.GetAuthMiddleware("")))
		apiV1.GET("/users/self", h.echoHandler(h.GetSelfUser), echo.WrapMiddleware(h.GetAuthMiddleware("")))

		// role routes
		apiV1.POST("/roles", h.echoHandler(h.CreateRole), echo.WrapMiddleware(h.GetAuthMiddleware(domain.RoleCrete)))
		apiV1.PUT("/roles", h.echoHandler(h.UpdateRole), echo.WrapMiddleware(h.GetAuthMiddleware(domain.RoleUpdate)))
		apiV1.DELETE("/roles", h.echoHandler(h.DeleteRole), echo.WrapMiddleware(h.GetAuthMiddleware(domain.RoleDelete)))
		apiV1.GET("/roles", h.echoHandler(h.ListRoles), echo.WrapMiddleware(h.GetAuthMiddleware(domain.RoleRead)))
		apiV1.GET("/permissions", h.echoHandler(h.ListPermissions), echo.WrapMiddleware(h.GetAuthMiddleware(domain.PermissionRead)))

		// strategy routes
		apiV1.POST("/strategies", h.echoHandler(h.CreateScheduleStrategy), echo.WrapMiddleware(h.GetAuthMiddleware(domain.ScheduleStrategyCreate)))
		apiV1.GET("/strategies/self", h.echoHandler(h.ListSelfScheduleStrategies), echo.WrapMiddleware(h.GetAuthMiddleware(domain.ScheduleStrategyRead)))
		apiV1.GET("/intents/self", h.echoHandler(h.ListSelfScheduleIntents), echo.WrapMiddleware(h.GetAuthMiddleware(domain.ScheduleIntentRead)))
	}

}

func (h *Handler) echoHandler(handlerFunc func(w http.ResponseWriter, r *http.Request)) echo.HandlerFunc {
	return echo.WrapHandler(http.HandlerFunc(handlerFunc))
}
