package domain

import (
	"time"

	"github.com/Gthulhu/api/pkg/util"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type EncryptedPassword string

func (value EncryptedPassword) MarshalBSONValue() (typ byte, data []byte, err error) {
	pwdHash, err := util.CreateArgon2Hash(string(value))
	return byte(bson.TypeString), []byte(pwdHash), err
}

func (value *EncryptedPassword) UnmarshalBSONValue(typ byte, data []byte) error {
	*value = EncryptedPassword(string(data))
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
	CreatedTime int64         `bson:"created_time,omitempty"`
	UpdatedTime int64         `bson:"updated_time,omitempty"`
	DeletedTime int64         `bson:"deleted_time,omitempty"`
	CreatorID   bson.ObjectID `bson:"creator_id,omitempty"`
	UpdaterID   bson.ObjectID `bson:"updater_id,omitempty"`
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
