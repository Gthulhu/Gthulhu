package rest

import (
	"errors"
	"net/http"

	"github.com/Gthulhu/api/manager/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type CreateUserRequest struct {
	UserName string `json:"username"`
	Password string `json:"password"`
}

// CreateUser godoc
// @Summary Create user
// @Description Create a new user account.
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateUserRequest true "User payload"
// @Success 200 {object} SuccessResponse[EmptyResponse]
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/users [post]
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req CreateUserRequest
	err := h.JSONBind(r, &req)
	if err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	claims, ok := h.GetClaimsFromContext(ctx)
	if !ok {
		h.ErrorResponse(ctx, w, http.StatusUnauthorized, "Unauthorized", errors.New("claims not found"))
		return
	}

	err = h.Svc.CreateNewUser(ctx, &claims, req.UserName, req.Password)
	if err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	response := NewSuccessResponse[string](nil)
	h.JSONResponse(ctx, w, http.StatusOK, response)
}

type LoginRequest struct {
	UserName string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

// Login godoc
// @Summary User login
// @Description Authenticate user and return JWT token.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login payload"
// @Success 200 {object} SuccessResponse[LoginResponse]
// @Failure 400 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/auth/login [post]
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req LoginRequest
	err := h.JSONBind(r, &req)
	if err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Invalid request body", err)
		return
	}
	if req.UserName == "" || req.Password == "" {
		h.ErrorResponse(ctx, w, http.StatusUnprocessableEntity, "Username and password are required", errors.New("username or password is empty"))
		return
	}

	token, err := h.Svc.Login(ctx, req.UserName, req.Password)
	if err != nil {
		h.HandleError(ctx, w, err)
		return
	}
	respData := LoginResponse{
		Token: token,
	}
	response := NewSuccessResponse(&respData)
	h.JSONResponse(ctx, w, http.StatusOK, response)
}

type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

// ChangePassword godoc
// @Summary Change own password
// @Description Update current user's password.
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body ChangePasswordRequest true "Password payload"
// @Success 200 {object} SuccessResponse[EmptyResponse]
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/users/self/password [put]
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req ChangePasswordRequest
	err := h.JSONBind(r, &req)
	if err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	claims, ok := h.GetClaimsFromContext(ctx)
	if !ok {
		h.ErrorResponse(ctx, w, http.StatusUnauthorized, "Unauthorized", errors.New("claims not found"))
		return
	}
	if claims.UID == "" {
		h.ErrorResponse(ctx, w, http.StatusUnauthorized, "Unauthorized", errors.New("uid not found in claims"))
		return
	}

	err = h.Svc.ChangePassword(ctx, &claims, req.OldPassword, req.NewPassword)
	if err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	response := NewSuccessResponse[string](nil)
	h.JSONResponse(ctx, w, http.StatusOK, response)
}

type ResetPasswordRequest struct {
	UserID      string `json:"userID"`
	NewPassword string `json:"newPassword"`
}

// ResetPassword godoc
// @Summary Reset user password
// @Description Reset another user's password.
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body ResetPasswordRequest true "Reset payload"
// @Success 200 {object} SuccessResponse[EmptyResponse]
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/users/password [put]
func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req ResetPasswordRequest
	err := h.JSONBind(r, &req)
	if err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	claims, ok := h.GetClaimsFromContext(ctx)
	if !ok {
		h.ErrorResponse(ctx, w, http.StatusUnauthorized, "Unauthorized", errors.New("claims not found"))
		return
	}
	err = h.VerifyResourcePolicy(ctx, req.UserID)
	if err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	err = h.Svc.ResetPassword(ctx, &claims, req.UserID, req.NewPassword)
	if err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	response := NewSuccessResponse[string](nil)
	h.JSONResponse(ctx, w, http.StatusOK, response)
}

type UpdateUserPermissionsRequest struct {
	UserID string             `json:"userID"`
	Roles  *[]string          `json:"roles,omitempty"`
	Status *domain.UserStatus `json:"status,omitempty"`
}

// UpdateUserPermissions godoc
// @Summary Update user roles and status
// @Description Update a user's roles or status.
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body UpdateUserPermissionsRequest true "Permissions payload"
// @Success 200 {object} SuccessResponse[EmptyResponse]
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/users/permissions [put]
func (h *Handler) UpdateUserPermissions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req UpdateUserPermissionsRequest
	err := h.JSONBind(r, &req)
	if err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Invalid request body", err)
	}
	claims, ok := h.GetClaimsFromContext(ctx)
	if !ok {
		h.ErrorResponse(ctx, w, http.StatusUnauthorized, "Unauthorized", errors.New("claims not found"))
		return
	}
	err = h.VerifyResourcePolicy(ctx, req.UserID)
	if err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	err = h.Svc.UpdateUserPermissions(ctx, &claims, req.UserID, domain.UpdateUserPermissionsOptions{
		Roles:  req.Roles,
		Status: req.Status,
	})
	if err != nil {
		h.HandleError(ctx, w, err)
		return
	}
	response := NewSuccessResponse[string](nil)
	h.JSONResponse(ctx, w, http.StatusOK, response)
}

type ListUsersResponse struct {
	Users []struct {
		ID       string            `json:"id"`
		UserName string            `json:"username"`
		Roles    []string          `json:"roles"`
		Status   domain.UserStatus `json:"status"`
	} `json:"users"`
}

// ListUsers godoc
// @Summary List users
// @Description Retrieve user list.
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Success 200 {object} SuccessResponse[ListUsersResponse]
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/users [get]
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := domain.QueryUserOptions{}
	err := h.Svc.QueryUsers(ctx, &query)
	if err != nil {
		h.HandleError(ctx, w, err)
		return
	}
	respData := ListUsersResponse{}
	for _, user := range query.Result {
		userInfo := struct {
			ID       string            `json:"id"`
			UserName string            `json:"username"`
			Roles    []string          `json:"roles"`
			Status   domain.UserStatus `json:"status"`
		}{
			ID:       user.ID.Hex(),
			UserName: user.UserName,
			Status:   user.Status,
		}
		for _, role := range user.Roles {
			userInfo.Roles = append(userInfo.Roles, role)
		}
		respData.Users = append(respData.Users, userInfo)
	}
	response := NewSuccessResponse(&respData)
	h.JSONResponse(ctx, w, http.StatusOK, response)
}

type GetSelfUserResponse struct {
	ID       string            `json:"id"`
	UserName string            `json:"username"`
	Roles    []string          `json:"roles"`
	Status   domain.UserStatus `json:"status"`
}

// GetSelfUser godoc
// @Summary Get current user
// @Description Retrieve profile of the authenticated user.
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Success 200 {object} SuccessResponse[GetSelfUserResponse]
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/users/self [get]
func (h *Handler) GetSelfUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims, ok := h.GetClaimsFromContext(ctx)
	if !ok {
		h.ErrorResponse(ctx, w, http.StatusUnauthorized, "Unauthorized", errors.New("claims not found"))
		return
	}
	uid, err := claims.GetBsonObjectUID()
	if err != nil {
		h.ErrorResponse(ctx, w, http.StatusUnauthorized, "Unauthorized", errors.New("invalid user ID in claims"))
		return
	}
	query := domain.QueryUserOptions{
		IDs: []bson.ObjectID{uid},
	}
	err = h.Svc.QueryUsers(ctx, &query)
	if err != nil {
		h.HandleError(ctx, w, err)
		return
	}
	if len(query.Result) == 0 {
		h.ErrorResponse(ctx, w, http.StatusUnauthorized, "Unauthorized", errors.New("user not found"))
		return
	}
	user := query.Result[0]
	respData := GetSelfUserResponse{
		ID:       user.ID.Hex(),
		UserName: user.UserName,
		Status:   user.Status,
	}
	for _, role := range user.Roles {
		respData.Roles = append(respData.Roles, role)
	}
	response := NewSuccessResponse(&respData)
	h.JSONResponse(ctx, w, http.StatusOK, response)
}
