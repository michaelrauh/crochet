package main

import (
	"crochet/types"
	"fmt"
	"time"

	"github.com/dgraph-io/ristretto"
)

// OrthosCache provides an interface for caching orthos
type OrthosCache interface {
	// FilterNewOrthos filters out orthos that are already in the cache
	FilterNewOrthos(orthos []types.Ortho) []types.Ortho
}

// RistrettoOrthosCache implements OrthosCache using Ristretto
type RistrettoOrthosCache struct {
	cache *ristretto.Cache
}

// NewRistrettoOrthosCache creates a new orthos cache using Ristretto
func NewRistrettoOrthosCache() (*RistrettoOrthosCache, error) {
	config := &ristretto.Config{
		NumCounters: 1e7,
		MaxCost:     1 << 30,
		BufferItems: 64,
		Metrics:     true,
	}
	cache, err := ristretto.NewCache(config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize orthos cache: %w", err)
	}
	return &RistrettoOrthosCache{
		cache: cache,
	}, nil
}

// FilterNewOrthos filters out orthos that are already in the cache
func (c *RistrettoOrthosCache) FilterNewOrthos(orthos []types.Ortho) []types.Ortho {
	newOrthos := make([]types.Ortho, 0)
	for _, ortho := range orthos {
		if _, found := c.cache.Get(ortho.ID); !found {
			newOrthos = append(newOrthos, ortho)
			c.cache.SetWithTTL(ortho.ID, true, 1, 24*time.Hour)
		}
	}
	c.cache.Wait()
	return newOrthos
}
