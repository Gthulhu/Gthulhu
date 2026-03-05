package rest

import (
	"context"
	"net/http"

	docs "github.com/Gthulhu/api/docs/manager"
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
		apiV1.PUT("/strategies", h.echoHandler(h.UpdateScheduleStrategy), echo.WrapMiddleware(h.GetAuthMiddleware(domain.ScheduleStrategyUpdate)))
		apiV1.GET("/strategies/self", h.echoHandler(h.ListSelfScheduleStrategies), echo.WrapMiddleware(h.GetAuthMiddleware(domain.ScheduleStrategyRead)))
		apiV1.DELETE("/strategies", h.echoHandler(h.DeleteScheduleStrategy), echo.WrapMiddleware(h.GetAuthMiddleware(domain.ScheduleStrategyDelete)))
		apiV1.GET("/intents/self", h.echoHandler(h.ListSelfScheduleIntents), echo.WrapMiddleware(h.GetAuthMiddleware(domain.ScheduleIntentRead)))
		apiV1.DELETE("/intents", h.echoHandler(h.DeleteScheduleIntents), echo.WrapMiddleware(h.GetAuthMiddleware(domain.ScheduleIntentDelete)))

		// pod-pid mapping routes
		apiV1.GET("/nodes", h.echoHandler(h.ListNodes), echo.WrapMiddleware(h.GetAuthMiddleware(domain.PodPIDMappingRead)))
		apiV1.GET("/nodes/:nodeID/pods/pids", h.echoHandlerWithParams(h.GetNodePodPIDMapping), echo.WrapMiddleware(h.GetAuthMiddleware(domain.PodPIDMappingRead)))
	}

}

func (h *Handler) echoHandler(handlerFunc func(w http.ResponseWriter, r *http.Request)) echo.HandlerFunc {
	return echo.WrapHandler(http.HandlerFunc(handlerFunc))
}

// echoHandlerWithParams wraps a handler function and injects path parameters into request context
func (h *Handler) echoHandlerWithParams(handlerFunc func(w http.ResponseWriter, r *http.Request)) echo.HandlerFunc {
	return func(c echo.Context) error {
		r := c.Request()
		// Store path params in request context
		for _, name := range c.ParamNames() {
			r = r.WithContext(context.WithValue(r.Context(), pathParamKey(name), c.Param(name)))
		}
		handlerFunc(c.Response().Writer, r)
		return nil
	}
}

// pathParamKey is a type for path parameter context keys
type pathParamKey string

// GetPathParam retrieves a path parameter from request context
func (h *Handler) GetPathParam(r *http.Request, name string) string {
	if val, ok := r.Context().Value(pathParamKey(name)).(string); ok {
		return val
	}
	return ""
}
