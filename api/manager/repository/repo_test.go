package repository

import (
	"context"
	"testing"

	"github.com/Gthulhu/api/config"
	"github.com/Gthulhu/api/manager/domain"
	"github.com/Gthulhu/api/pkg/container"
	"github.com/Gthulhu/api/pkg/logger"
	"github.com/Gthulhu/api/pkg/util"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(RepositoryTestSuite))
}

type RepositoryTestSuite struct {
	suite.Suite
	ctx            context.Context
	repo           *repo
	containerBuild *container.ContainerBuilder
	mongoCfg       config.MongoDBConfig
}

func (suite *RepositoryTestSuite) SetupSuite() {
	logger.InitLogger()
	suite.ctx = context.Background()

	builder, err := container.NewContainerBuilder("")
	suite.Require().NoError(err, "init container builder")
	suite.containerBuild = builder

	cfg, err := config.InitManagerConfig("manager_config.test.toml", config.GetAbsPath("config"))
	suite.Require().NoError(err, "load test config")
	cfg.MongoDB.Port = "27018"

	conn, err := container.RunMongoContainer(builder, "api_repo_test_mongo", container.MongoContainerConnection{
		Username: cfg.MongoDB.User,
		Password: string(cfg.MongoDB.Password),
		Database: cfg.MongoDB.Database,
		Port:     cfg.MongoDB.Port,
	})
	suite.Require().NoError(err, "start mongo container")

	cfg.MongoDB.Host = conn.Host
	cfg.MongoDB.Port = conn.Port
	cfg.MongoDB.User = conn.Username
	cfg.MongoDB.Password = config.SecretValue(conn.Password)
	cfg.MongoDB.Database = conn.Database
	suite.mongoCfg = cfg.MongoDB

	repoInst, err := NewRepository(Params{MongoConfig: cfg.MongoDB})
	suite.Require().NoError(err, "init repository")

	r, ok := repoInst.(*repo)
	suite.Require().True(ok, "repository type assertion")
	suite.repo = r
}

func (suite *RepositoryTestSuite) TearDownSuite() {
	if suite.containerBuild != nil {
		err := suite.containerBuild.PruneAll()
		suite.Require().NoError(err, "prune containers")
	}
}

func (suite *RepositoryTestSuite) SetupTest() {
	suite.Require().NotNil(suite.repo, "repository not initialized")
	err := util.MongoCleanup(suite.repo.client, suite.mongoCfg.Database)
	suite.Require().NoError(err, "cleanup database")
}

func (suite *RepositoryTestSuite) TestCreateAndQueryUser() {
	user := &domain.User{
		UserName: "test-user",
		Password: domain.EncryptedPassword("secret"),
		Status:   domain.UserStatusActive,
	}
	err := suite.repo.CreateUser(suite.ctx, user)
	suite.Require().NoError(err, "create user")
	suite.NotZero(user.ID, "user id should be assigned")

	opts := &domain.QueryUserOptions{
		UserNames: []string{user.UserName},
	}
	err = suite.repo.QueryUsers(suite.ctx, opts)
	suite.Require().NoError(err, "query users")
	suite.Len(opts.Result, 1, "expect one user")
	suite.Equal(user.UserName, opts.Result[0].UserName, "username should match")
}

func (suite *RepositoryTestSuite) TestUpdateUserStatus() {
	user := &domain.User{
		UserName: "update-user",
		Password: domain.EncryptedPassword("secret"),
		Status:   domain.UserStatusActive,
	}
	err := suite.repo.CreateUser(suite.ctx, user)
	suite.Require().NoError(err, "create user")

	user.Status = domain.UserStatusInactive
	err = suite.repo.UpdateUser(suite.ctx, user)
	suite.Require().NoError(err, "update user")

	opts := &domain.QueryUserOptions{IDs: []bson.ObjectID{user.ID}}
	err = suite.repo.QueryUsers(suite.ctx, opts)
	suite.Require().NoError(err, "query users by id")
	suite.Len(opts.Result, 1, "expect one user after update")
	suite.Equal(domain.UserStatusInactive, opts.Result[0].Status, "status should be updated")
}

func (suite *RepositoryTestSuite) TestCreateRoleAndPermission() {
	role := &domain.Role{
		Name:        "viewer",
		Description: "view only",
		Policies: []domain.RolePolicy{
			{
				PermissionKey: domain.PermissionRead,
				Self:          true,
			},
		},
	}
	err := suite.repo.CreateRole(suite.ctx, role)
	suite.Require().NoError(err, "create role")

	roleOpts := &domain.QueryRoleOptions{Names: []string{role.Name}}
	err = suite.repo.QueryRoles(suite.ctx, roleOpts)
	suite.Require().NoError(err, "query roles")
	suite.Len(roleOpts.Result, 1, "expect one role")
	suite.Equal(role.Name, roleOpts.Result[0].Name, "role name should match")

	perm := &domain.Permission{
		Key:         domain.PermissionRead,
		Description: "read access",
		Resource:    "resource",
		Action:      domain.PermissionActionRead,
	}
	err = suite.repo.CreatePermission(suite.ctx, perm)
	suite.Require().NoError(err, "create permission")

	permOpts := &domain.QueryPermissionOptions{Keys: []string{string(perm.Key)}}
	err = suite.repo.QueryPermissions(suite.ctx, permOpts)
	suite.Require().NoError(err, "query permissions")
	suite.Len(permOpts.Result, 1, "expect one permission")
	suite.Equal(perm.Description, permOpts.Result[0].Description, "permission description should match")
}
