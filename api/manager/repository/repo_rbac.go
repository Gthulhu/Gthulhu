package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Gthulhu/api/manager/domain"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func (r *repo) CreateUser(ctx context.Context, user *domain.User) error {
	if user == nil {
		return errors.New("nil user")
	}

	now := time.Now().UnixMilli()
	if user.ID.IsZero() {
		user.ID = bson.NewObjectID()
	}
	if user.CreatedTime == 0 {
		user.CreatedTime = now
	}
	user.UpdatedTime = now

	res, err := r.db.Collection(userCollection).InsertOne(ctx, user)
	if err != nil {
		return fmt.Errorf("create user, err: %w", err)
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		user.ID = oid
	}
	return nil
}

func (r *repo) UpdateUser(ctx context.Context, user *domain.User) error {
	if user == nil {
		return errors.New("nil user")
	}
	if user.ID.IsZero() {
		return errors.New("user id is required")
	}

	user.UpdatedTime = time.Now().UnixMilli()
	res, err := r.db.Collection(userCollection).ReplaceOne(ctx, bson.M{"_id": user.ID}, user)
	if err != nil {
		return fmt.Errorf("update user, err: %w", err)
	}
	if res.MatchedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *repo) QueryUsers(ctx context.Context, opt *domain.QueryUserOptions) error {
	if opt == nil {
		return errors.New("nil query options")
	}

	filter := bson.M{}
	if len(opt.IDs) > 0 {
		filter["_id"] = bson.M{"$in": opt.IDs}
	}
	if len(opt.UserNames) > 0 {
		filter["username"] = bson.M{"$in": opt.UserNames}
	}

	cursor, err := r.db.Collection(userCollection).Find(ctx, filter)
	if err != nil {
		return fmt.Errorf("find users, err: %w", err)
	}

	var result []*domain.User
	if err := cursor.All(ctx, &result); err != nil {
		return fmt.Errorf("decode users, err: %w", err)
	}
	opt.Result = result
	return nil
}

func (r *repo) CreateRole(ctx context.Context, role *domain.Role) error {
	if role == nil {
		return errors.New("nil role")
	}

	now := time.Now().UnixMilli()
	if role.ID.IsZero() {
		role.ID = bson.NewObjectID()
	}
	if role.CreatedTime == 0 {
		role.CreatedTime = now
	}
	role.UpdatedTime = now

	res, err := r.db.Collection(roleCollection).InsertOne(ctx, role)
	if err != nil {
		return fmt.Errorf("create role, err: %w", err)
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		role.ID = oid
	}
	return nil
}

func (r *repo) UpdateRole(ctx context.Context, role *domain.Role) error {
	if role == nil {
		return errors.New("nil role")
	}
	if role.ID.IsZero() {
		return errors.New("role id is required")
	}

	role.UpdatedTime = time.Now().UnixMilli()
	res, err := r.db.Collection(roleCollection).ReplaceOne(ctx, bson.M{"_id": role.ID}, role)
	if err != nil {
		return fmt.Errorf("update role, err: %w", err)
	}
	if res.MatchedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *repo) QueryRoles(ctx context.Context, opt *domain.QueryRoleOptions) error {
	if opt == nil {
		return errors.New("nil query options")
	}

	filter := bson.M{}
	if len(opt.IDs) > 0 {
		filter["_id"] = bson.M{"$in": opt.IDs}
	}
	if len(opt.Names) > 0 {
		filter["name"] = bson.M{"$in": opt.Names}
	}

	cursor, err := r.db.Collection(roleCollection).Find(ctx, filter)
	if err != nil {
		return fmt.Errorf("find roles, err: %w", err)
	}

	var result []*domain.Role
	if err := cursor.All(ctx, &result); err != nil {
		return fmt.Errorf("decode roles, err: %w", err)
	}
	opt.Result = result
	return nil
}

func (r *repo) CreatePermission(ctx context.Context, permission *domain.Permission) error {
	if permission == nil {
		return errors.New("nil permission")
	}

	if permission.ID.IsZero() {
		permission.ID = bson.NewObjectID()
	}

	res, err := r.db.Collection(permissionCollection).InsertOne(ctx, permission)
	if err != nil {
		return fmt.Errorf("create permission, err: %w", err)
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		permission.ID = oid
	}
	return nil
}

func (r *repo) UpdatePermission(ctx context.Context, permission *domain.Permission) error {
	if permission == nil {
		return errors.New("nil permission")
	}
	if permission.ID.IsZero() {
		return errors.New("permission id is required")
	}

	res, err := r.db.Collection(permissionCollection).ReplaceOne(ctx, bson.M{"_id": permission.ID}, permission)
	if err != nil {
		return fmt.Errorf("update permission, err: %w", err)
	}
	if res.MatchedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *repo) QueryPermissions(ctx context.Context, opt *domain.QueryPermissionOptions) error {
	if opt == nil {
		return errors.New("nil query options")
	}

	filter := bson.M{}
	if len(opt.IDs) > 0 {
		filter["_id"] = bson.M{"$in": opt.IDs}
	}
	if len(opt.Keys) > 0 {
		filter["key"] = bson.M{"$in": opt.Keys}
	}
	if len(opt.Resources) > 0 {
		filter["resource"] = bson.M{"$in": opt.Resources}
	}

	cursor, err := r.db.Collection(permissionCollection).Find(ctx, filter)
	if err != nil {
		return fmt.Errorf("find permissions, err: %w", err)
	}

	var result []*domain.Permission
	if err := cursor.All(ctx, &result); err != nil {
		return fmt.Errorf("decode permissions, err: %w", err)
	}
	opt.Result = result
	return nil
}

func (r *repo) CreateAuditLog(ctx context.Context, log *domain.AuditLog) error {
	if log == nil {
		return errors.New("nil audit log")
	}
	if log.ID.IsZero() {
		log.ID = bson.NewObjectID()
	}
	if log.Timestamp == 0 {
		log.Timestamp = time.Now().UnixMilli()
	}

	res, err := r.db.Collection(auditLogCollection).InsertOne(ctx, log)
	if err != nil {
		return fmt.Errorf("create audit log, err: %w", err)
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		log.ID = oid
	}
	return nil
}

func (r *repo) QueryAuditLogs(ctx context.Context, opt *domain.QueryAuditLogOptions) error {
	if opt == nil {
		return errors.New("nil query options")
	}

	filter := bson.M{}
	if len(opt.UserIDs) > 0 {
		filter["user_id"] = bson.M{"$in": opt.UserIDs}
	}

	if opt.TimestampGTE > 0 || opt.TimestampLTE > 0 {
		timeFilter := bson.M{}
		if opt.TimestampGTE > 0 {
			timeFilter["$gte"] = opt.TimestampGTE
		}
		if opt.TimestampLTE > 0 {
			timeFilter["$lte"] = opt.TimestampLTE
		}
		filter[defaultTimestampField] = timeFilter
	}

	cursor, err := r.db.Collection(auditLogCollection).Find(ctx, filter)
	if err != nil {
		return fmt.Errorf("find audit logs, err: %w", err)
	}

	var result []*domain.AuditLog
	if err := cursor.All(ctx, &result); err != nil {
		return fmt.Errorf("decode audit logs, err: %w", err)
	}
	opt.Result = result
	return nil
}
