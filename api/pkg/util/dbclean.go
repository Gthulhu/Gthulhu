package util

import "go.mongodb.org/mongo-driver/v2/mongo"

func MongoCleanup(mongodbClient *mongo.Client, dbName string) error {
	return mongodbClient.Database(dbName).Drop(nil)
}
