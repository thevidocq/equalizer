package session

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis is a store shared by all gateway replicas.
type Redis struct {
	client *redis.Client
}

// NewRedis connects to the Redis at url (redis://host:6379/0).
func NewRedis(url string) (*Redis, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	opts.DialTimeout = 2 * time.Second
	opts.ReadTimeout = time.Second
	opts.WriteTimeout = time.Second
	return &Redis{client: redis.NewClient(opts)}, nil
}

// Ping checks the connection.
func (r *Redis) Ping(ctx context.Context) error { return r.client.Ping(ctx).Err() }

// Load implements Store.
func (r *Redis) Load(ctx context.Context, key string) ([]byte, error) {
	b, err := r.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrNotFound
	}
	return b, err
}

// Save implements Store.
func (r *Redis) Save(ctx context.Context, key string, blob []byte, ttl time.Duration) error {
	return r.client.Set(ctx, key, blob, ttl).Err()
}

// Delete implements Store.
func (r *Redis) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

// Close releases the connections.
func (r *Redis) Close() error { return r.client.Close() }
