package rest_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Gthulhu/api/config"
	"github.com/Gthulhu/api/manager/app"
	"github.com/Gthulhu/api/manager/rest"
	"github.com/Gthulhu/api/pkg/container"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"
)

func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

type HandlerTestSuite struct {
	suite.Suite
	Handler *rest.Handler
	Ctx     context.Context
	Engine  *echo.Echo
	*container.ContainerBuilder
}

func (suite *HandlerTestSuite) SetupSuite() {
	suite.Ctx = context.Background()
	containerBuilder, err := container.NewContainerBuilder("")
	suite.Require().NoError(err, "Failed to create container builder")
	suite.ContainerBuilder = containerBuilder

	cfg, err := config.InitManagerConfig("manager_config.test.toml", config.GetAbsPath("config"))
	suite.Require().NoError(err, "Failed to initialize manager config")

	repoModule, err := app.TestRepoModule(cfg, suite.ContainerBuilder)
	suite.Require().NoError(err, "Failed to create repo module")

	serviceModule, err := app.ServiceModule(repoModule)
	suite.Require().NoError(err, "Failed to create service module")

	handlerModule, err := app.HandlerModule(serviceModule)
	suite.Require().NoError(err, "Failed to create handler module")
	opt := fx.Options(
		handlerModule,
		fx.Populate(&suite.Handler),
	)

	err = fx.New(opt).Start(suite.Ctx)
	suite.Require().NoError(err, "Failed to start Fx app")
	suite.Require().NotNil(suite.Handler, "Handler should not be nil")
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	suite.Engine = e
	suite.Handler.SetupRoutes(e)
}

func (suite *HandlerTestSuite) TearDownSuite() {
	err := suite.ContainerBuilder.PruneAll()
	suite.Require().NoError(err, "Failed to terminate containers")
}

func (suite *HandlerTestSuite) JSONDecode(r *httptest.ResponseRecorder, dst any) {
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(dst)
	suite.Require().NoError(err, "Failed to decode JSON response")
}

func (suite *HandlerTestSuite) TestHealthCheck() {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	suite.Engine.ServeHTTP(rec, req)

	suite.Equal(http.StatusOK, rec.Code, "Expected status OK")
	var resp map[string]any
	suite.JSONDecode(rec, &resp)
	suite.Equal("healthy", resp["status"].(string), "Expected status to be healthy")
}
