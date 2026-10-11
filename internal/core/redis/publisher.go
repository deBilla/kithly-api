// Package redis provides asynq client, server, inspector, and a Redis pub/sub
// publisher for broadcasting transcoding lifecycle events.
package redis

import (
	"context"
	"encoding/json"
	"fmt"

	goredis "github.com/redis/go-redis/v9"
	"github.com/kithly/kithly-ws-go/internal/core/config"
)

// Event channel names mirror the TypeScript EVENT_CHANNELS constants so that
// existing subscribers (e.g. the TS listener) continue to receive events.
const (
	ChannelTranscodingCompleted = "video.transcoding.completed"
	ChannelTranscodingFailed    = "video.transcoding.failed"
)

// Publisher wraps a go-redis client to publish JSON events to Redis pub/sub channels.
// It is safe for concurrent use.
type Publisher struct {
	rdb *goredis.Client
}

// NewPublisher constructs a Publisher using credentials from cfg.
func NewPublisher(cfg *config.Config) *Publisher {
	addr := fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort)
	opts := &goredis.Options{Addr: addr}
	if cfg.RedisPassword != "" {
		opts.Password = cfg.RedisPassword
	}
	return &Publisher{rdb: goredis.NewClient(opts)}
}

// Close releases the underlying Redis connection.
func (p *Publisher) Close() error {
	return p.rdb.Close()
}

// Publish serialises payload as JSON and publishes it to channel.
// Returns an error if marshalling or the Redis PUBLISH command fails.
func (p *Publisher) Publish(ctx context.Context, channel string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("redis.Publisher.Publish: marshal: %w", err)
	}
	if err := p.rdb.Publish(ctx, channel, data).Err(); err != nil {
		return fmt.Errorf("redis.Publisher.Publish %q: %w", channel, err)
	}
	return nil
}
