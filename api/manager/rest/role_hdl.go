package rest

import (
	"errors"
	"net/http"

	"github.com/Gthulhu/api/manager/domain"
)

type RolePolicy struct {
	PermissionKey   domain.PermissionKey `json:"permissionKey"`
	Self            bool                 `json:"self"`
	K8SNamespace    string               `json:"k8sNamespace"`
	PolicyNamespace string               `json:"policyNamespace"`
}

type CreateRoleRequest struct {
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	RolePolicies []RolePolicy `json:"rolePolicies"`
}

// CreateRole godoc
// @Summary Create role
// @Description Create a new role with policies.
// @Tags Roles
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateRoleRequest true "Role payload"
// @Success 200 {object} SuccessResponse[EmptyResponse]
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/roles [post]
func (h *Handler) CreateRole(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req CreateRoleRequest
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
	role := domain.Role{
		Name:        req.Name,
		Description: req.Description,
	}
	for _, rp := range req.RolePolicies {
		role.Policies = append(role.Policies, domain.RolePolicy{
			PermissionKey:   rp.PermissionKey,
			Self:            rp.Self,
			K8SNamespace:    rp.K8SNamespace,
			PolicyNamespace: rp.PolicyNamespace,
		})
	}

	err = h.Svc.CreateRole(ctx, &claims, &role)
	if err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	response := NewSuccessResponse[string](nil)
	h.JSONResponse(ctx, w, http.StatusOK, response)
}

type UpdateRoleRequest struct {
	ID          string        `json:"id"`
	Name        *string       `json:"name,omitempty"`
	Description *string       `json:"description,omitempty"`
	RolePolicy  *[]RolePolicy `json:"rolePolicy,omitempty"`
}

// UpdateRole godoc
// @Summary Update role
// @Description Update role information or policies.
// @Tags Roles
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body UpdateRoleRequest true "Role payload"
// @Success 200 {object} SuccessResponse[EmptyResponse]
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/roles [put]
func (h *Handler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req UpdateRoleRequest
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

	updateOpts := domain.UpdateRoleOptions{}
	if req.Name != nil {
		updateOpts.Name = req.Name
	}
	if req.Description != nil {
		updateOpts.Description = req.Description
	}
	if req.RolePolicy != nil {
		var policies []domain.RolePolicy
		for _, rp := range *req.RolePolicy {
			policies = append(policies, domain.RolePolicy{
				PermissionKey:   rp.PermissionKey,
				Self:            rp.Self,
				K8SNamespace:    rp.K8SNamespace,
				PolicyNamespace: rp.PolicyNamespace,
			})
		}
		updateOpts.Policies = &policies
	}

	err = h.Svc.UpdateRole(ctx, &claims, req.ID, updateOpts)
	if err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	response := NewSuccessResponse[string](nil)
	h.JSONResponse(ctx, w, http.StatusOK, response)
}

type DeleteRoleRequest struct {
	ID string `json:"id"`
}

// DeleteRole godoc
// @Summary Delete role
// @Description Delete a role by ID.
// @Tags Roles
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body DeleteRoleRequest true "Role payload"
// @Success 200 {object} SuccessResponse[EmptyResponse]
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/roles [delete]
func (h *Handler) DeleteRole(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req DeleteRoleRequest
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

	err = h.Svc.DeleteRole(ctx, &claims, req.ID)
	if err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	response := NewSuccessResponse[string](nil)
	h.JSONResponse(ctx, w, http.StatusOK, response)
}

type ListRolesResponse struct {
	Roles []struct {
		ID          string       `json:"id"`
		Name        string       `json:"name"`
		Description string       `json:"description"`
		RolePolicy  []RolePolicy `json:"rolePolicy"`
	} `json:"roles"`
}

// ListRoles godoc
// @Summary List roles
// @Description Retrieve available roles.
// @Tags Roles
// @Produce json
// @Security BearerAuth
// @Success 200 {object} SuccessResponse[ListRolesResponse]
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/roles [get]
func (h *Handler) ListRoles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	queryOpts := &domain.QueryRoleOptions{}
	err := h.Svc.QueryRoles(ctx, queryOpts)
	if err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	var resp ListRolesResponse
	for _, role := range queryOpts.Result {
		r := struct {
			ID          string       `json:"id"`
			Name        string       `json:"name"`
			Description string       `json:"description"`
			RolePolicy  []RolePolicy `json:"rolePolicy"`
		}{
			ID:          role.ID.Hex(),
			Name:        role.Name,
			Description: role.Description,
		}
		for _, rp := range role.Policies {
			r.RolePolicy = append(r.RolePolicy, RolePolicy{
				PermissionKey:   rp.PermissionKey,
				Self:            rp.Self,
				K8SNamespace:    rp.K8SNamespace,
				PolicyNamespace: rp.PolicyNamespace,
			})
		}
		resp.Roles = append(resp.Roles, r)
	}

	response := NewSuccessResponse[ListRolesResponse](&resp)
	h.JSONResponse(ctx, w, http.StatusOK, response)

}

type ListPermissionsResponse struct {
	Permissions []struct {
		Key         domain.PermissionKey `json:"key"`
		Description string               `json:"description"`
	} `json:"permissions"`
}

// ListPermissions godoc
// @Summary List permissions
// @Description Retrieve all permission keys.
// @Tags Roles
// @Produce json
// @Security BearerAuth
// @Success 200 {object} SuccessResponse[ListPermissionsResponse]
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/permissions [get]
func (h *Handler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	queryOpts := &domain.QueryPermissionOptions{}
	err := h.Svc.QueryPermissions(ctx, queryOpts)
	if err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	var resp ListPermissionsResponse
	for _, perm := range queryOpts.Result {
		p := struct {
			Key         domain.PermissionKey `json:"key"`
			Description string               `json:"description"`
		}{
			Key:         perm.Key,
			Description: perm.Description,
		}
		resp.Permissions = append(resp.Permissions, p)
	}

	response := NewSuccessResponse[ListPermissionsResponse](&resp)
	h.JSONResponse(ctx, w, http.StatusOK, response)
}
