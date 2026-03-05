package domain

import (
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Claims represents JWT token claims
type Claims struct {
	UID                string `json:"uid"`
	NeedChangePassword bool   `json:"needChangePassword"`
	jwt.RegisteredClaims
}

func (c *Claims) GetBsonObjectUID() (bson.ObjectID, error) {
	return bson.ObjectIDFromHex(c.UID)
}
