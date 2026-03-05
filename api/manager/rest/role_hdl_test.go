package rest_test

import (
	"net/http"

	"github.com/Gthulhu/api/config"
	"github.com/Gthulhu/api/manager/domain"
	"github.com/Gthulhu/api/manager/rest"
	"github.com/Gthulhu/api/pkg/util"
)

func (suite *HandlerTestSuite) TestIntegrationRoleHandler() {
	adminUser, adminPwd := config.GetManagerConfig().Account.AdminEmail, config.GetManagerConfig().Account.AdminPassword
	adminToken := suite.login(adminUser, adminPwd.Value(), http.StatusOK)

	roleManager := "role_manager"
	rolePolicies := []rest.RolePolicy{
		{
			PermissionKey:   domain.RoleRead,
			Self:            false,
			K8SNamespace:    "",
			PolicyNamespace: "",
		},
	}
	suite.createRole("", roleManager, rolePolicies, http.StatusUnauthorized)
	suite.createRole(adminToken, roleManager, rolePolicies, http.StatusOK)
	suite.listRoles(adminToken, http.StatusOK, 2)

	newUserName, newUserPwd := "rolemanager", "rolemanagerpwd"
	suite.createUser(adminToken, newUserName, newUserPwd, http.StatusOK)
	users := suite.listUsers(adminToken, http.StatusOK, 2)
	newUserUID := ""
	for _, r := range users.Users {
		if r.UserName == newUserName {
			newUserUID = r.ID
			break
		}
	}
	suite.Require().NotEmpty(newUserUID, "Newly created role not found in list roles response")

	changePwdToken := suite.login(newUserName, newUserPwd, http.StatusOK)
	newUserNewPwd := "newrolemanagerpwd"
	suite.changePassword(changePwdToken, newUserPwd, newUserNewPwd, http.StatusOK)
	userToken := suite.login(newUserName, newUserNewPwd, http.StatusOK)
	suite.listRoles(userToken, http.StatusForbidden, 0)

	suite.updateUserPermissions(adminToken, rest.UpdateUserPermissionsRequest{UserID: newUserUID, Roles: util.Ptr([]string{roleManager})}, http.StatusOK)
	roles := suite.listRoles(userToken, http.StatusOK, 2)
	roleID := ""
	for _, r := range roles.Roles {
		if r.Name == roleManager {
			roleID = r.ID
			break
		}
	}
	suite.Require().NotEmpty(roleID, "Role not found in list roles response")
	newPolicy := []rest.RolePolicy{
		{
			PermissionKey:   domain.RoleUpdate,
			Self:            false,
			K8SNamespace:    "",
			PolicyNamespace: "",
		},
	}
	suite.updateRole(userToken, roleID, newPolicy, http.StatusForbidden)
	suite.updateRole(adminToken, roleID, newPolicy, http.StatusOK)

	suite.listPermissions(userToken, http.StatusForbidden)
	suite.updateRole(userToken, roleID, []rest.RolePolicy{{PermissionKey: domain.PermissionRead}}, http.StatusOK)
	permissions := suite.listPermissions(userToken, http.StatusOK)
	suite.Require().Greater(len(permissions.Permissions), 0, "Expected non-zero permissions")
}

func (suite *HandlerTestSuite) createRole(token, roleName string, policies []rest.RolePolicy, expectedStatus int) {
	createRoleReq := rest.CreateRoleRequest{
		Name:         roleName,
		RolePolicies: policies,
	}
	createRoleResp := rest.SuccessResponse[string]{}
	_, resp := suite.sendV1Request("POST", "/roles", createRoleReq, &createRoleResp, token)
	suite.Require().Equal(expectedStatus, resp.Code, "Unexpected status code on create role")
}

func (suite *HandlerTestSuite) listRoles(token string, expectedStatus int, expectedRoleCnt int) rest.ListRolesResponse {
	listRolesResp := rest.SuccessResponse[rest.ListRolesResponse]{}
	_, resp := suite.sendV1Request("GET", "/roles", nil, &listRolesResp, token)
	suite.Require().Equal(expectedStatus, resp.Code, "Unexpected status code on list roles")
	if expectedStatus == http.StatusOK {
		suite.Equal(expectedRoleCnt, len(listRolesResp.Data.Roles), "Unexpected number of roles returned")
		return *listRolesResp.Data
	}
	return rest.ListRolesResponse{}
}

func (suite *HandlerTestSuite) updateRole(token, roleID string, rolePolicies []rest.RolePolicy, expectedStatus int) {
	updateRoleReq := rest.UpdateRoleRequest{
		ID:         roleID,
		RolePolicy: util.Ptr(rolePolicies),
	}
	updateRoleResp := rest.SuccessResponse[struct{}]{}
	_, resp := suite.sendV1Request("PUT", "/roles", updateRoleReq, &updateRoleResp, token)
	suite.Require().Equal(expectedStatus, resp.Code, "Unexpected status code on update role")
}

func (suite *HandlerTestSuite) listPermissions(token string, expectedStatus int) rest.ListPermissionsResponse {
	listPermissionsResp := rest.SuccessResponse[rest.ListPermissionsResponse]{}
	_, resp := suite.sendV1Request("GET", "/permissions", nil, &listPermissionsResp, token)
	suite.Require().Equal(expectedStatus, resp.Code, "Unexpected status code on list permissions")
	if expectedStatus == http.StatusOK {
		return *listPermissionsResp.Data
	}
	return rest.ListPermissionsResponse{}
}
