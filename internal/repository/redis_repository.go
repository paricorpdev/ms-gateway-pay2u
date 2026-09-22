package repository

import (
	"context"
	"errors"
	"time"

	fiberredis "github.com/gofiber/storage/redis/v3"
	goredis "github.com/redis/go-redis/v9"
)

type redisRepository struct {
	client goredis.UniversalClient
}

// NewCacheRepository builds the Redis-backed cache adapter.
//
// It uses the underlying go-redis client rather than the Fiber storage
// wrapper, because only the former accepts a context.
func NewCacheRepository(storage *fiberredis.Storage) CacheRepository {
	return &redisRepository{client: storage.Conn()}
}

// Set stores a value with a time to live; a non-positive ttl is clamped so no
// session entry becomes immortal.
func (r *redisRepository) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = time.Second
	}
	return r.client.Set(ctx, key, value, ttl).Err()
}

// Get returns (nil, nil) when the key is absent, so callers can distinguish
// "no session" from "cache is broken".
func (r *redisRepository) Get(ctx context.Context, key string) ([]byte, error) {
	value, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, nil
		}
		return nil, err
	}
	return value, nil
}

func (r *redisRepository) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}
