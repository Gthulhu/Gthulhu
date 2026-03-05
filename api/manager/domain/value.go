package domain

import (
	"fmt"
	"time"

	"github.com/Gthulhu/api/pkg/util"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/x/bsonx/bsoncore"
)

type EncryptedPassword string

func (value EncryptedPassword) MarshalBSONValue() (typ byte, data []byte, err error) {
	valStr := string(value)
	if util.IsArgon2Hash(valStr) {
		return byte(bson.TypeString), bsoncore.AppendString(nil, valStr), nil
	}
	pwdHash, err := util.CreateArgon2Hash(valStr)
	return byte(bson.TypeString), bsoncore.AppendString(nil, string(pwdHash)), err
}

func (value *EncryptedPassword) UnmarshalBSONValue(typ byte, data []byte) error {
	if typ != byte(bson.TypeString) {
		return fmt.Errorf("invalid type %v for EncryptedPassword", bson.Type(typ))
	}

	str, _, ok := bsoncore.ReadString(data)
	if !ok {
		return fmt.Errorf("failed to read bson string")
	}

	*value = EncryptedPassword(str)
	return nil
}

func (value EncryptedPassword) String() string {
	return "*******"
}

func (value EncryptedPassword) Cmp(plainText string) (bool, error) {
	ok, err := util.ComparePasswordAndHash(plainText, string(value))
	if err != nil {
		return false, err
	}
	return ok, nil
}

type BaseEntity struct {
	ID          bson.ObjectID `bson:"_id,omitempty"`
	CreatedTime int64         `bson:"createdTime,omitempty"`
	UpdatedTime int64         `bson:"updatedTime,omitempty"`
	DeletedTime int64         `bson:"deletedTime,omitempty"`
	CreatorID   bson.ObjectID `bson:"creatorID,omitempty"`
	UpdaterID   bson.ObjectID `bson:"updaterID,omitempty"`
}

func NewBaseEntity(creatorID, updaterID *bson.ObjectID) BaseEntity {
	nowInmsec := time.Now().UnixMilli()
	entity := BaseEntity{
		CreatedTime: nowInmsec,
		UpdatedTime: nowInmsec,
		DeletedTime: 0,
	}
	if creatorID != nil {
		entity.CreatorID = *creatorID
	}
	if updaterID != nil {
		entity.UpdaterID = *updaterID
	}
	return entity
}
