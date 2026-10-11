// Package redis provides asynq client, server, and inspector constructors
// configured from the application Config. All queue consumers and producers
// must obtain their connections through this package.
package redis

import (
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/kithly/kithly-ws-go/internal/core/config"
)

// redisOpts builds the asynq RedisClientOpt from Config.
func redisOpts(cfg *config.Config) asynq.RedisClientOpt {
	addr := fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort)
	if cfg.RedisPassword != "" {
		return asynq.RedisClientOpt{Addr: addr, Password: cfg.RedisPassword}
	}
	return asynq.RedisClientOpt{Addr: addr}
}

// NewClient returns an asynq client for enqueuing tasks.
// The caller is responsible for calling Close() when done.
func NewClient(cfg *config.Config) *asynq.Client {
	return asynq.NewClient(redisOpts(cfg))
}

// NewServer returns an asynq server for consuming tasks.
// concurrency controls how many tasks are processed in parallel.
// queues maps queue name → priority weight (e.g. map[string]int{"video-processing": 1}).
// If queues is nil or empty the server falls back to asynq's default ("default" queue only).
func NewServer(cfg *config.Config, concurrency int, queues map[string]int) *asynq.Server {
	return asynq.NewServer(redisOpts(cfg), asynq.Config{
		Concurrency: concurrency,
		Queues:      queues,
		// Retry up to 3 times with exponential backoff.
		RetryDelayFunc: asynq.DefaultRetryDelayFunc,
	})
}

// NewInspector returns an asynq inspector for queue introspection
// (used by the dashboard service).
func NewInspector(cfg *config.Config) *asynq.Inspector {
	return asynq.NewInspector(redisOpts(cfg))
}
