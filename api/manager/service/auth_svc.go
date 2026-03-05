package service

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Gthulhu/api/manager/domain"
	"github.com/Gthulhu/api/manager/errs"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (svc *Service) CreateNewUser(ctx context.Context, operator *domain.Claims, username, password string) error {
	operatorID, err := operator.GetBsonObjectUID()
	if err != nil {
		return errors.WithMessagef(err, "invalid operator ID %s", operator.UID)
	}
	user := &domain.User{
		UserName:   username,
		Password:   domain.EncryptedPassword(password),
		Status:     domain.UserStatusWaitChangePassword,
		BaseEntity: domain.NewBaseEntity(&operatorID, &operatorID),
	}
	err = svc.Repo.CreateUser(ctx, user)
	if err != nil {
		return errors.WithMessagef(err, "db: create user %s failed", username)
	}
	return nil
}

func (svc *Service) Login(ctx context.Context, username, password string) (string, error) {
	user, err := svc.getUserByUserName(ctx, username)
	if err != nil {
		return "", err
	}
	if user.Status == domain.UserStatusInactive {
		return "", errs.NewHTTPStatusError(http.StatusUnauthorized, "user is inactive", fmt.Errorf("username %s is inactive", username))
	}

	ok, err := user.Password.Cmp(password)
	if err != nil {
		return "", errors.WithMessagef(err, "compare password for username %s failed", username)
	}
	if !ok {
		return "", errs.NewHTTPStatusError(http.StatusUnauthorized, "invalid password", fmt.Errorf("compare password for username %s not match", username))
	}
	token, err := svc.genJWTToken(ctx, user)
	if err != nil {
		return "", errors.WithMessage(err, "generate JWT token failed")
	}
	return token, nil
}

func (svc *Service) ChangePassword(ctx context.Context, userClaims *domain.Claims, oldPassword, newPassword string) error {
	uid, err := userClaims.GetBsonObjectUID()
	if err != nil {
		return errors.WithMessagef(err, "invalid user ID %s", userClaims.UID)
	}

	user, err := svc.getUserByID(ctx, uid)
	if err != nil {
		return err
	}
	ok, err := user.Password.Cmp(oldPassword)
	if err != nil {
		return errors.WithMessagef(err, "compare password for uid %s failed", uid)
	}
	if !ok {
		return errs.NewHTTPStatusError(http.StatusUnauthorized, "invalid password", fmt.Errorf("change password failed, compare password for uid %s not match", uid))
	}
	user.Status = domain.UserStatusActive
	user.Password = domain.EncryptedPassword(newPassword)
	user.UpdatedTime = time.Now().UnixMilli()
	user.UpdaterID = uid
	err = svc.Repo.UpdateUser(ctx, user)
	if err != nil {
		return err
	}
	return nil
}

func (svc *Service) UpdateUserPermissions(ctx context.Context, operator *domain.Claims, id string, opt domain.UpdateUserPermissionsOptions) error {
	operatorID, err := operator.GetBsonObjectUID()
	if err != nil {
		return errors.WithMessagef(err, "invalid operator ID %s", operator.UID)
	}
	uid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return errs.NewHTTPStatusError(http.StatusUnprocessableEntity, "invalid user ID", fmt.Errorf("invalid user ID %s: %v", id, err))
	}

	user, err := svc.getUserByID(ctx, uid)
	if err != nil {
		return err
	}
	if opt.Roles != nil {
		query := &domain.QueryRoleOptions{
			Names: *opt.Roles,
		}
		err = svc.QueryRoles(ctx, query)
		if err != nil {
			return err
		}
		if len(*opt.Roles) != len(query.Result) {
			return errs.NewHTTPStatusError(http.StatusBadRequest, "Some roles not found", errors.New("invalid role names"))
		}
		user.Roles = *opt.Roles
	}
	if opt.Status != nil {
		user.Status = *opt.Status
	}
	user.UpdatedTime = time.Now().UnixMilli()
	user.UpdaterID = operatorID
	err = svc.Repo.UpdateUser(ctx, user)
	if err != nil {
		return err
	}
	return nil
}

func (svc *Service) ResetPassword(ctx context.Context, operator *domain.Claims, id, newPassword string) error {
	operatorID, err := operator.GetBsonObjectUID()
	if err != nil {
		return errors.WithMessagef(err, "invalid operator ID %s", operator.UID)
	}
	uid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return errs.NewHTTPStatusError(http.StatusUnprocessableEntity, "invalid user ID", fmt.Errorf("invalid user ID %s: %v", id, err))
	}

	user, err := svc.getUserByID(ctx, uid)
	if err != nil {
		return err
	}
	user.Password = domain.EncryptedPassword(newPassword)
	user.Status = domain.UserStatusWaitChangePassword
	user.UpdatedTime = time.Now().UnixMilli()
	user.UpdaterID = operatorID
	err = svc.Repo.UpdateUser(ctx, user)
	if err != nil {
		return err
	}
	return nil
}

func (svc *Service) QueryUsers(ctx context.Context, opt *domain.QueryUserOptions) error {
	err := svc.Repo.QueryUsers(ctx, opt)
	if err != nil {
		return err
	}
	return nil
}

func (svc *Service) getUserByUserName(ctx context.Context, username string) (*domain.User, error) {
	opts := &domain.QueryUserOptions{
		UserNames: []string{username},
	}
	err := svc.Repo.QueryUsers(ctx, opts)
	if err != nil {
		return nil, err
	}
	users := opts.Result
	if len(users) == 0 {
		return nil, errs.NewHTTPStatusError(http.StatusUnauthorized, "user not found", fmt.Errorf("username %s not found", username))
	}

	return users[0], nil
}

func (svc *Service) getUserByID(ctx context.Context, id bson.ObjectID) (*domain.User, error) {
	opts := &domain.QueryUserOptions{
		IDs: []bson.ObjectID{id},
	}
	err := svc.Repo.QueryUsers(ctx, opts)
	if err != nil {
		return nil, err
	}
	users := opts.Result
	if len(users) == 0 {
		return nil, errs.NewHTTPStatusError(http.StatusUnauthorized, "user not found", fmt.Errorf("user ID %s not found", id.Hex()))
	}

	return users[0], nil
}

func (svc *Service) genJWTToken(ctx context.Context, user *domain.User) (string, error) {
	tokenTTL := time.Duration(3) * time.Hour
	uid := user.ID.Hex()

	roles := []string{}
	for _, role := range user.Roles {
		roles = append(roles, role)
	}
	claims := domain.Claims{
		UID:                uid,
		NeedChangePassword: user.Status == domain.UserStatusWaitChangePassword,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "bss-api-server",
			Subject:   uid,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(svc.jwtPrivateKey)
}

func (svc *Service) VerifyJWTToken(ctx context.Context, tokenString string, permissionKey domain.PermissionKey) (domain.Claims, domain.RolePolicy, error) {
	token, err := jwt.ParseWithClaims(tokenString, &domain.Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return svc.jwtPrivateKey.Public(), nil
	})
	if err != nil {
		return domain.Claims{}, domain.RolePolicy{}, errors.WithMessage(err, "parse JWT token failed")
	}
	claims, ok := token.Claims.(*domain.Claims)
	if !ok || !token.Valid {
		return domain.Claims{}, domain.RolePolicy{}, errors.New("invalid JWT token claims")
	}
	if permissionKey == "" {
		return *claims, domain.RolePolicy{}, nil
	}
	if permissionKey != domain.ChangeUserPermission && claims.NeedChangePassword {
		return domain.Claims{}, domain.RolePolicy{}, errs.NewHTTPStatusError(http.StatusForbidden, "password change required", fmt.Errorf("user %s need to change password", claims.UID))
	}

	uid, err := claims.GetBsonObjectUID()
	if err != nil {
		return domain.Claims{}, domain.RolePolicy{}, errors.WithMessagef(err, "invalid user ID %s", claims.UID)
	}
	user, err := svc.getUserByID(ctx, uid)
	if err != nil {
		return domain.Claims{}, domain.RolePolicy{}, errors.WithMessagef(err, "get user by ID %s failed", uid.Hex())
	}

	roles, err := svc.getRolesByNames(ctx, user.Roles)
	if err != nil {
		return domain.Claims{}, domain.RolePolicy{}, errors.WithMessage(err, "get roles by IDs failed")
	}
	if len(roles) == 0 {
		return domain.Claims{}, domain.RolePolicy{}, errs.NewHTTPStatusError(http.StatusForbidden, "permission denied", fmt.Errorf("user %s has no roles assigned", claims.UID))
	}
	hasPermission := false
	rolePolicy := domain.RolePolicy{}
	for _, role := range roles {
		for _, policy := range role.Policies {
			if policy.PermissionKey == permissionKey {
				hasPermission = true
				rolePolicy = policy
				break
			}
		}
		if hasPermission {
			break
		}
	}
	if !hasPermission {
		return domain.Claims{}, domain.RolePolicy{}, errs.NewHTTPStatusError(http.StatusForbidden, "permission denied", fmt.Errorf("user %s does not have permission %s", claims.UID, permissionKey))
	}
	return *claims, rolePolicy, nil
}

func (svc *Service) CreateAdminUserIfNotExists(ctx context.Context, username, password string) error {
	opts := &domain.QueryUserOptions{
		UserNames: []string{username},
	}
	err := svc.Repo.QueryUsers(ctx, opts)
	if err != nil {
		return errors.WithMessagef(err, "db: query user %s failed", username)
	}
	if len(opts.Result) > 0 {
		return nil
	}
	roleOpts := &domain.QueryRoleOptions{
		Names: []string{domain.AdminRole},
	}
	err = svc.QueryRoles(ctx, roleOpts)
	if err != nil {
		return errors.WithMessagef(err, "db: query admin role failed")
	}
	if len(roleOpts.Result) == 0 {
		return errors.New("admin role not found, please create admin role first")
	}

	adminUser := &domain.User{
		UserName:   username,
		Password:   domain.EncryptedPassword(password),
		Status:     domain.UserStatusActive,
		Roles:      []string{domain.AdminRole},
		BaseEntity: domain.NewBaseEntity(nil, nil),
	}
	err = svc.Repo.CreateUser(ctx, adminUser)
	if err != nil {
		return errors.WithMessagef(err, "db: create admin user %s failed", username)
	}
	return nil
}
