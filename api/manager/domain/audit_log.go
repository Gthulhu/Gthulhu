package domain

import "go.mongodb.org/mongo-driver/v2/bson"

type AuditLog struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	UserID    bson.ObjectID `bson:"user_id,omitempty"`
	Action    string        `bson:"action,omitempty"`
	RequestID string        `bson:"request_id,omitempty"`
	Timestamp int64         `bson:"timestamp,omitempty"`
	IP        string        `bson:"ip,omitempty"`
}
