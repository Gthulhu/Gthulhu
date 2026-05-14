package repository

import (
	"context"
	"errors"

	"github.com/Gthulhu/api/manager/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const nodeRuntimeConfigCollection = "node_runtime_configs"

func (r *repo) UpsertNodeRuntimeConfig(ctx context.Context, cfg *domain.NodeRuntimeConfig) error {
	if cfg == nil {
		return errors.New("nil node runtime config")
	}
	_, err := r.db.Collection(nodeRuntimeConfigCollection).ReplaceOne(
		ctx,
		bson.M{"nodeId": cfg.NodeID},
		cfg,
		options.Replace().SetUpsert(true),
	)
	return err
}

func (r *repo) QueryNodeRuntimeConfigs(ctx context.Context, opt *domain.QueryNodeRuntimeConfigOptions) error {
	if opt == nil {
		return errors.New("nil query options")
	}
	filter := bson.M{}
	if len(opt.NodeIDs) > 0 {
		filter["nodeId"] = bson.M{"$in": opt.NodeIDs}
	}
	cursor, err := r.db.Collection(nodeRuntimeConfigCollection).Find(ctx, filter)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)
	var result []*domain.NodeRuntimeConfig
	if err := cursor.All(ctx, &result); err != nil {
		return err
	}
	opt.Result = append(opt.Result, result...)
	return nil
}
