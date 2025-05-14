package main

import (
	"context"
	"crochet/pkg/db"
)

// QueriesInterface defines the interface for database queries
// @mockery
type QueriesInterface interface {
	CreateItem(ctx context.Context, name string) (db.Item, error)
	GetItemByID(ctx context.Context, id int32) (db.Item, error)
}
