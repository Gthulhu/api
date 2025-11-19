package domain

import "go.mongodb.org/mongo-driver/v2/bson"

type EncryptedPassword string

type BaseEntity struct {
	ID          bson.ObjectID `bson:"_id,omitempty"`
	CreatedTime int64         `bson:"created_time,omitempty"`
	UpdatedTime int64         `bson:"updated_time,omitempty"`
	DeletedTime int64         `bson:"deleted_time,omitempty"`
	CreatorID   bson.ObjectID `bson:"creator_id,omitempty"`
	UpdaterID   bson.ObjectID `bson:"updater_id,omitempty"`
}
