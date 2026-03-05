package service

import (
	"context"
	"time"

	"github.com/Gthulhu/api/manager/domain"
	"github.com/Gthulhu/api/pkg/util"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (svc *Service) CreateUser(ctx context.Context, operator domain.Claims, user *domain.User) error {
	operatorID, err := bson.ObjectIDFromHex(operator.UID)
	if err != nil {
		return err
	}

	user.BaseEntity = domain.NewBaseEntity(util.Ptr(operatorID), util.Ptr(operatorID))
	user.Status = domain.UserStatusWaitChangePassword
	return svc.Repo.CreateUser(ctx, user)

}

func (svc *Service) DeleteUser(ctx context.Context, operator domain.Claims, userID bson.ObjectID) error {
	operatorID, err := bson.ObjectIDFromHex(operator.UID)
	if err != nil {
		return err
	}
	updateUser := &domain.User{
		BaseEntity: domain.NewBaseEntity(nil, util.Ptr(operatorID)),
	}
	updateUser.ID = userID
	updateUser.DeletedTime = time.Now().UnixMilli()
	err = svc.Repo.UpdateUser(ctx, updateUser)
	if err != nil {
		return err
	}
	return nil
}

func (svc *Service) UpdateUser(ctx context.Context, operator domain.Claims, user *domain.User) error {
	operatorID, err := bson.ObjectIDFromHex(operator.UID)
	if err != nil {
		return err
	}
	user.UpdaterID = operatorID
	user.UpdatedTime = time.Now().UnixMilli()
	err = svc.Repo.UpdateUser(ctx, user)
	if err != nil {
		return err
	}
	return nil
}
