package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Gthulhu/api/manager/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (r *repo) InsertStrategyAndIntents(ctx context.Context, strategy *domain.ScheduleStrategy, intents []*domain.ScheduleIntent) error {
	if strategy == nil {
		return errors.New("nil strategy")
	}
	if intents == nil {
		return errors.New("nil intents")
	}
	now := time.Now().UnixMilli()
	if strategy.CreatedTime == 0 {
		strategy.CreatedTime = now
	}
	strategy.UpdatedTime = now
	res, err := r.db.Collection(scheduleStrategyCollection).InsertOne(ctx, strategy)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		strategy.ID = oid
	}

	for _, intent := range intents {
		if intent.ID.IsZero() {
			intent.ID = bson.NewObjectID()
		}
		intent.StrategyID = strategy.ID
		if intent.CreatedTime == 0 {
			intent.CreatedTime = now
		}
		if intent.UpdatedTime == 0 {
			intent.UpdatedTime = now
		}
	}
	_, err = r.db.Collection(scheduleIntentCollection).InsertMany(ctx, intents)
	if err != nil {
		return err
	}
	return nil
}

func (r *repo) InsertIntents(ctx context.Context, intents []*domain.ScheduleIntent) error {
	if len(intents) == 0 {
		return nil
	}
	now := time.Now().UnixMilli()
	for _, intent := range intents {
		if intent.ID.IsZero() {
			intent.ID = bson.NewObjectID()
		}
		if intent.CreatedTime == 0 {
			intent.CreatedTime = now
		}
		if intent.UpdatedTime == 0 {
			intent.UpdatedTime = now
		}
	}
	_, err := r.db.Collection(scheduleIntentCollection).InsertMany(ctx, intents)
	return err
}

func (r *repo) BatchUpdateIntentsState(ctx context.Context, intentIDs []bson.ObjectID, newState domain.IntentState) error {
	update := bson.M{
		"$set": bson.M{
			"state":      newState,
			"updateTime": time.Now().UnixMilli(),
		},
	}
	_, err := r.db.Collection(scheduleIntentCollection).UpdateMany(ctx, bson.M{
		"_id": bson.M{"$in": intentIDs},
	}, update)
	if err != nil {
		return err
	}
	return nil
}

func (r *repo) QueryStrategies(ctx context.Context, opt *domain.QueryStrategyOptions) error {
	if opt == nil {
		return errors.New("nil query options")
	}
	filter := bson.M{}
	if len(opt.IDs) > 0 {
		filter["_id"] = bson.M{"$in": opt.IDs}
	}
	if len(opt.K8SNamespaces) > 0 {
		filter["k8sNamespace"] = bson.M{"$in": opt.K8SNamespaces}
	}
	if len(opt.CreatorIDs) > 0 {
		filter["creatorID"] = bson.M{"$in": opt.CreatorIDs}
	}
	cursor, err := r.db.Collection(scheduleStrategyCollection).Find(ctx, filter)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var strategy domain.ScheduleStrategy
		if err := cursor.Decode(&strategy); err != nil {
			return err
		}
		opt.Result = append(opt.Result, &strategy)
	}
	return cursor.Err()
}

func (r *repo) QueryIntents(ctx context.Context, opt *domain.QueryIntentOptions) error {
	if opt == nil {
		return errors.New("nil query options")
	}
	filter := bson.M{}
	if len(opt.IDs) > 0 {
		filter["_id"] = bson.M{"$in": opt.IDs}
	}
	if len(opt.K8SNamespaces) > 0 {
		filter["k8sNamespace"] = bson.M{"$in": opt.K8SNamespaces}
	}
	if len(opt.StrategyIDs) > 0 {
		filter["strategyID"] = bson.M{"$in": opt.StrategyIDs}
	}
	if len(opt.PodIDs) > 0 {
		filter["podID"] = bson.M{"$in": opt.PodIDs}
	}
	if len(opt.States) > 0 {
		filter["state"] = bson.M{"$in": opt.States}
	}
	if len(opt.CreatorIDs) > 0 {
		filter["creatorID"] = bson.M{"$in": opt.CreatorIDs}
	}
	cursor, err := r.db.Collection(scheduleIntentCollection).Find(ctx, filter)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var intent domain.ScheduleIntent
		if err := cursor.Decode(&intent); err != nil {
			return err
		}
		opt.Result = append(opt.Result, &intent)
	}
	return cursor.Err()
}

func (r *repo) DeleteStrategy(ctx context.Context, strategyID bson.ObjectID) error {
	_, err := r.db.Collection(scheduleStrategyCollection).DeleteOne(ctx, bson.M{"_id": strategyID})
	return err
}

func (r *repo) UpdateStrategy(ctx context.Context, strategyID bson.ObjectID, update bson.M) error {
	if update == nil {
		return errors.New("nil update")
	}
	_, err := r.db.Collection(scheduleStrategyCollection).UpdateOne(ctx, bson.M{"_id": strategyID}, update)
	return err
}

func (r *repo) DeleteIntents(ctx context.Context, intentIDs []bson.ObjectID) error {
	if len(intentIDs) == 0 {
		return nil
	}
	_, err := r.db.Collection(scheduleIntentCollection).DeleteMany(ctx, bson.M{"_id": bson.M{"$in": intentIDs}})
	return err
}

func (r *repo) DeleteIntentsByStrategyID(ctx context.Context, strategyID bson.ObjectID) error {
	_, err := r.db.Collection(scheduleIntentCollection).DeleteMany(ctx, bson.M{"strategyID": strategyID})
	return err
}
