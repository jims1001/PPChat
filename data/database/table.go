package database

import "go.mongodb.org/mongo-driver/mongo"

type Table interface {
	GetTableName() string
	Collection() *mongo.Collection
}
