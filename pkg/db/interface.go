package db

import (
	"context"
)

type QueriesInterface interface {
	CreateItem(ctx context.Context, name string) (Item, error)
	GetItemByID(ctx context.Context, id int32) (Item, error)
}

var _ QueriesInterface = (*Queries)(nil)
