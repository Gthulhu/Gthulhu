package service

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Gthulhu/api/manager/domain"
	"github.com/Gthulhu/api/manager/errs"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (svc *Service) CreateRole(ctx context.Context, operator *domain.Claims, role *domain.Role) error {
	operatorID, err := operator.GetBsonObjectUID()
	if err != nil {
		return errs.NewHTTPStatusError(http.StatusUnauthorized, "unauthorized", fmt.Errorf("invalid user ID"))
	}
	role.BaseEntity = domain.NewBaseEntity(&operatorID, &operatorID)
	return svc.Repo.CreateRole(ctx, role)
}

func (svc *Service) UpdateRole(ctx context.Context, operator *domain.Claims, roleID string, opt domain.UpdateRoleOptions) error {
	operatorID, err := operator.GetBsonObjectUID()
	if err != nil {
		return errs.NewHTTPStatusError(http.StatusUnauthorized, "unauthorized", fmt.Errorf("invalid user ID"))
	}
	roles, err := svc.getRolesByIDs(ctx, []string{roleID})
	if err != nil {
		return err
	}
	if len(roles) == 0 {
		return errs.NewHTTPStatusError(http.StatusUnprocessableEntity, "role not found", fmt.Errorf("role with ID %s not found", roleID))
	}
	role := roles[0]
	if opt.Name != nil {
		role.Name = *opt.Name
	}
	if opt.Description != nil {
		role.Description = *opt.Description
	}
	if opt.Policies != nil {
		role.Policies = []domain.RolePolicy{}
		for _, p := range *opt.Policies {
			role.Policies = append(role.Policies, domain.RolePolicy{
				PermissionKey:   p.PermissionKey,
				Self:            p.Self,
				K8SNamespace:    p.K8SNamespace,
				PolicyNamespace: p.PolicyNamespace,
			})
		}
	}
	role.UpdaterID = operatorID
	return svc.Repo.UpdateRole(ctx, role)
}

func (svc *Service) DeleteRole(ctx context.Context, operator *domain.Claims, roleID string) error {
	return fmt.Errorf("not implemented")
}

func (svc *Service) QueryRoles(ctx context.Context, opt *domain.QueryRoleOptions) error {
	return svc.Repo.QueryRoles(ctx, opt)
}

func (svc *Service) getRolesByNames(ctx context.Context, roleNames []string) ([]*domain.Role, error) {
	if len(roleNames) == 0 {
		return []*domain.Role{}, nil
	}
	opts := &domain.QueryRoleOptions{
		Names: roleNames,
	}
	err := svc.Repo.QueryRoles(ctx, opts)
	if err != nil {
		return nil, err
	}
	return opts.Result, nil
}
func (svc *Service) getRolesByIDs(ctx context.Context, roleIDs []string) ([]*domain.Role, error) {
	if len(roleIDs) == 0 {
		return []*domain.Role{}, nil
	}
	ids := []bson.ObjectID{}
	for _, idStr := range roleIDs {
		id, err := bson.ObjectIDFromHex(idStr)
		if err != nil {
			return nil, errors.WithMessagef(err, "invalid role ID %s", idStr)
		}
		ids = append(ids, id)
	}
	opts := &domain.QueryRoleOptions{
		IDs: ids,
	}
	err := svc.Repo.QueryRoles(ctx, opts)
	if err != nil {
		return nil, err
	}
	return opts.Result, nil
}

func (svc Service) QueryPermissions(ctx context.Context, opt *domain.QueryPermissionOptions) error {
	return svc.Repo.QueryPermissions(ctx, opt)
}
