package domain

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Claims represents JWT token claims
type Claims struct {
	UID                string `json:"uid"`
	NeedChangePassword bool   `json:"needChangePassword"`
	TokenVersion       int64  `json:"tokenVersion"`
	TokenType          string `json:"tokenType"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

type RefreshSession struct {
	JTI       string `bson:"jti,omitempty"`
	TokenHash string `bson:"tokenHash,omitempty"`
	ExpiresAt int64  `bson:"expiresAt,omitempty"`
	Revoked   bool   `bson:"revoked,omitempty"`
}

func (s *RefreshSession) IsExpired(now time.Time) bool {
	return s.ExpiresAt > 0 && now.Unix() >= s.ExpiresAt
}

func (c *Claims) GetBsonObjectUID() (bson.ObjectID, error) {
	return bson.ObjectIDFromHex(c.UID)
}
