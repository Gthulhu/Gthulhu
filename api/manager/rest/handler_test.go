package rest_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Gthulhu/api/config"
	"github.com/Gthulhu/api/manager/app"
	"github.com/Gthulhu/api/manager/domain"
	"github.com/Gthulhu/api/manager/migration"
	"github.com/Gthulhu/api/manager/rest"
	"github.com/Gthulhu/api/pkg/container"
	"github.com/Gthulhu/api/pkg/logger"
	"github.com/Gthulhu/api/pkg/util"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
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
	mongoDBClient *mongo.Client
	mongoDBCfg    config.MongoDBConfig

	MockK8SAdapter *domain.MockK8SAdapter
	MockDMAdapter  *domain.MockDecisionMakerAdapter
}

func (suite *HandlerTestSuite) SetupSuite() {
	logger.InitLogger()
	suite.Ctx = context.Background()
	containerBuilder, err := container.NewContainerBuilder("")
	suite.Require().NoError(err, "Failed to create container builder")
	suite.ContainerBuilder = containerBuilder

	suite.MockK8SAdapter = domain.NewMockK8SAdapter(suite.T())
	suite.MockDMAdapter = domain.NewMockDecisionMakerAdapter(suite.T())

	cfg, err := config.InitManagerConfig("manager_config.test.toml", config.GetAbsPath("config"))
	suite.Require().NoError(err, "Failed to initialize manager config")

	repoModule, err := app.TestRepoModule(cfg, suite.ContainerBuilder)
	suite.Require().NoError(err, "Failed to create repo module")

	adapterModule := fx.Options(
		fx.Provide(func() domain.K8SAdapter {
			return suite.MockK8SAdapter
		}),
		fx.Provide(func() domain.DecisionMakerAdapter {
			return suite.MockDMAdapter
		}),
	)

	serviceModule, err := app.ServiceModule(adapterModule, repoModule)
	suite.Require().NoError(err, "Failed to create service module")

	handlerModule, err := app.HandlerModule(serviceModule)
	suite.Require().NoError(err, "Failed to create handler module")
	opt := fx.Options(
		handlerModule,
		fx.Invoke(migration.RunMongoMigration),
		fx.Populate(&suite.Handler),
		fx.Invoke(func(mongoDBCfg config.MongoDBConfig) {
			suite.mongoDBCfg = mongoDBCfg
			suite.newMongoClient()
		}),
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

func (suite *HandlerTestSuite) SetupTest() {
	err := util.MongoCleanup(suite.mongoDBClient, suite.mongoDBCfg.Database)
	suite.Require().NoError(err, "Failed to clean up MongoDB")
	err = migration.RunMongoMigration(suite.mongoDBCfg)
	suite.Require().NoError(err, "Failed to run MongoDB migrations")
	err = suite.Handler.Svc.CreateAdminUserIfNotExists(suite.Ctx, config.GetManagerConfig().Account.AdminEmail, config.GetManagerConfig().Account.AdminPassword.Value())
	suite.Require().NoError(err, "Failed to create admin user")
}

func (suite *HandlerTestSuite) TearDownSuite() {
	if os.Getenv("LOCAL_TEST") == "true" {
		return
	}
	err := suite.ContainerBuilder.PruneAll()
	suite.Require().NoError(err, "Failed to terminate containers")
}

func (suite *HandlerTestSuite) JSONDecode(r *httptest.ResponseRecorder, dst any) {
	rBody, err := io.ReadAll(r.Body)
	suite.Require().NoError(err, "Failed to read response body")
	err = json.Unmarshal(rBody, dst)
	suite.Require().NoErrorf(err, "Failed to decode JSON response, body: %s", string(rBody))
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

func (suite *HandlerTestSuite) sendV1Request(method, path string, reqStruct any, respStruct any, token string) (*http.Request, *httptest.ResponseRecorder) {
	reqBody := []byte{}
	if reqStruct != nil {
		var err error
		reqBody, err = json.Marshal(reqStruct)
		suite.Require().NoError(err, "Failed to marshal request body")
	}
	v1Path := "/api/v1" + path
	req := httptest.NewRequest(method, v1Path, bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	suite.Engine.ServeHTTP(rec, req)
	if respStruct != nil {
		suite.JSONDecode(rec, respStruct)
	}
	return req, rec
}

func (suite *HandlerTestSuite) newMongoClient() {
	uri := suite.mongoDBCfg.GetURI()
	mongoOpts := options.Client().ApplyURI(uri)
	if suite.mongoDBCfg.CAPem != "" {
		caPool := x509.NewCertPool()
		caPool.AppendCertsFromPEM([]byte(suite.mongoDBCfg.CAPem))
		tlsConfig := &tls.Config{
			RootCAs:            caPool,
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: false,
		}
		mongoOpts.SetTLSConfig(tlsConfig)
	}

	client, err := mongo.Connect(mongoOpts, nil)
	suite.Require().NoError(err, "Failed to connect to MongoDB")

	err = client.Ping(suite.Ctx, nil)
	suite.Require().NoError(err, "Failed to ping MongoDB")
	suite.mongoDBClient = client
}
