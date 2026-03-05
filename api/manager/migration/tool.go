package migration

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Gthulhu/api/config"
	"github.com/golang-migrate/migrate/v4"
	mongodbmigrate "github.com/golang-migrate/migrate/v4/database/mongodb"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func RunMongoMigration(mongodbCfg config.MongoDBConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	uri := mongodbCfg.GetURI()
	dbName := mongodbCfg.Database

	mongoOpts := options.Client().ApplyURI(uri)
	hasCa := false
	if mongodbCfg.CAPem != "" && mongodbCfg.CAPemEnable {
		hasCa = true
		caPool := x509.NewCertPool()
		caPool.AppendCertsFromPEM([]byte(mongodbCfg.CAPem))
		tlsConfig := &tls.Config{
			RootCAs:            caPool,
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: false,
		}
		mongoOpts.SetTLSConfig(tlsConfig)
	}
	client, err := mongo.Connect(ctx, mongoOpts)
	if err != nil {
		return fmt.Errorf("connect to mongodb: %w, uri:%s hasCa:%+v", err, uri, hasCa)
	}
	err = client.Ping(ctx, nil)
	if err != nil {
		return fmt.Errorf("ping mongodb: %w, uri:%s, hasCa:%+v", err, uri, hasCa)
	}

	driverConfig := &mongodbmigrate.Config{
		DatabaseName: dbName,
	}

	driver, err := mongodbmigrate.WithInstance(client, driverConfig)
	if err != nil {
		return fmt.Errorf("mongodb driver error: %w", err)
	}

	_, f, _, _ := runtime.Caller(0)
	dir := filepath.Dir(f)
	migrationsPath := dir
	m, err := migrate.NewWithDatabaseInstance(
		"file://"+migrationsPath,
		dbName,
		driver,
	)
	if err != nil {
		return fmt.Errorf("migrate init error: %w, uri:%s", err, uri)
	}

	err = m.Up()

	if err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up error: %w, uri:%s", err, uri)
	}

	return nil
}
