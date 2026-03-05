package rest_test

import (
	"net/http"

	"github.com/Gthulhu/api/config"
	"github.com/Gthulhu/api/manager/domain"
	"github.com/Gthulhu/api/manager/rest"
	"github.com/Gthulhu/api/pkg/util"
)

func (suite *HandlerTestSuite) TestIntegrationAuthHandler() {
	adminUser, adminPwd := config.GetManagerConfig().Account.AdminEmail, config.GetManagerConfig().Account.AdminPassword
	adminToken := suite.login(adminUser, adminPwd.Value(), http.StatusOK)

	// Test creating a new user
	newUsername := "testuser"
	newPassword := "testpassword"
	suite.createUser("", newUsername, newPassword, http.StatusUnauthorized) // without token should fail
	suite.createUser(adminToken, newUsername, newPassword, http.StatusOK)   // with admin token should succeed
	users := suite.listUsers(adminToken, http.StatusOK, 2)                  // should have 2 users now
	uid := ""
	for _, u := range users.Users {
		if u.UserName == newUsername {
			uid = u.ID
			break
		}
	}
	suite.updateUserPermissions(adminToken, rest.UpdateUserPermissionsRequest{
		UserID: uid,
		Roles:  util.Ptr([]string{domain.AdminRole}),
	}, http.StatusOK) // updating non-existing user should return not found

	// Get the user ID of the newly created user
	// Test login with the new user
	userToken := suite.login(newUsername, newPassword, http.StatusOK)
	suite.listUsers(userToken, http.StatusForbidden, 0) // password hasn't been changed yet, should be forbidden
	// Change password
	newUserPassword := "newtestpassword"
	suite.changePassword(userToken, newPassword, newUserPassword, http.StatusOK)
	// Login with old password should fail
	suite.login(newUsername, newPassword, http.StatusUnauthorized)
	// Login with new password should succeed
	userToken = suite.login(newUsername, newUserPassword, http.StatusOK)
	// Now listing users should succeed
	suite.listUsers(userToken, http.StatusOK, 2)
}

func (suite *HandlerTestSuite) login(username, password string, expectedStatus int) string {
	loginReq := rest.LoginRequest{
		UserName: username,
		Password: password,
	}
	loginResp := rest.SuccessResponse[rest.LoginResponse]{}

	_, resp := suite.sendV1Request("POST", "/auth/login", loginReq, &loginResp, "")
	suite.Require().Equal(expectedStatus, resp.Code, "Unexpected status code on login")
	if expectedStatus == http.StatusOK {
		suite.NotEmpty(loginResp.Data.Token, "Token should not be empty on successful login")
		return loginResp.Data.Token
	}
	return ""
}

func (suite *HandlerTestSuite) createUser(token, username, password string, expectedStatus int) {
	createUserReq := rest.CreateUserRequest{
		UserName: username,
		Password: password,
	}
	createUserResp := rest.SuccessResponse[string]{}

	_, resp := suite.sendV1Request("POST", "/users", createUserReq, &createUserResp, token)
	suite.Require().Equal(expectedStatus, resp.Code, "Unexpected status code on create user")
}

func (suite *HandlerTestSuite) listUsers(token string, expectedStatus int, expectedUserCnt int) rest.ListUsersResponse {
	listUserResp := rest.SuccessResponse[rest.ListUsersResponse]{}
	_, resp := suite.sendV1Request("GET", "/users", nil, &listUserResp, token)
	suite.Equal(expectedStatus, resp.Code, "Unexpected status code on list users")
	if expectedStatus == http.StatusOK {
		suite.Require().Equal(expectedUserCnt, len(listUserResp.Data.Users), "Unexpected number of users returned")
		return *listUserResp.Data
	}
	return rest.ListUsersResponse{}
}

func (suite *HandlerTestSuite) updateUserPermissions(token string, req rest.UpdateUserPermissionsRequest, expectedStatus int) {
	updatePermResp := rest.SuccessResponse[string]{}
	_, resp := suite.sendV1Request("PUT", "/users/permissions", req, &updatePermResp, token)
	suite.Require().Equal(expectedStatus, resp.Code, "Unexpected status code on update user permissions")
}

func (suite *HandlerTestSuite) changePassword(token, oldPassword, newPassword string, expectedStatus int) {
	changePwdReq := rest.ChangePasswordRequest{
		OldPassword: oldPassword,
		NewPassword: newPassword,
	}
	changePwdResp := rest.SuccessResponse[string]{}
	_, resp := suite.sendV1Request("PUT", "/users/self/password", changePwdReq, &changePwdResp, token)
	suite.Require().Equal(expectedStatus, resp.Code, "Unexpected status code on change password")
}
