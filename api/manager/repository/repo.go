package repository

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"time"

	"github.com/Gthulhu/api/config"
	"github.com/Gthulhu/api/manager/domain"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/fx"
)

type Params struct {
	fx.In
	MongoConfig config.MongoDBConfig
}

func NewRepository(params Params) (domain.Repository, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	uri := params.MongoConfig.GetURI()

	mongoOpts := options.Client().ApplyURI(uri)
	if params.MongoConfig.CAPem != "" && params.MongoConfig.CAPemEnable {
		caPool := x509.NewCertPool()
		caPool.AppendCertsFromPEM([]byte(params.MongoConfig.CAPem))
		tlsConfig := &tls.Config{
			RootCAs:            caPool,
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: false,
		}
		mongoOpts.SetTLSConfig(tlsConfig)
	}

	client, err := mongo.Connect(mongoOpts)
	if err != nil {
		return nil, fmt.Errorf("connect to mongodb: %w, uri:%s, tls:%+v", err, uri, params.MongoConfig.CAPemEnable)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("ping mongodb: %w, uri:%s, tls:%+v", err, uri, params.MongoConfig.CAPemEnable)
	}

	dbName := params.MongoConfig.Database
	if dbName == "" {
		dbName = "manager"
	}

	return &repo{
		client: client,
		db:     client.Database(dbName),
	}, nil
}

type repo struct {
	client *mongo.Client
	db     *mongo.Database
}

const (
	userCollection             = "users"
	roleCollection             = "roles"
	permissionCollection       = "permissions"
	auditLogCollection         = "audit_logs"
	defaultTimestampField      = "timestamp"
	scheduleStrategyCollection = "schedule_strategies"
	scheduleIntentCollection   = "schedule_intents"
)
