// Command dashboard runs the Asynqmon queue monitoring dashboard.
// It serves the asynqmon web UI on port 3002, providing visibility into
// all asynq queues, tasks, and workers — no authentication required
// (internal service only).
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/hibiken/asynq"
	"github.com/hibiken/asynqmon"

	"github.com/kithly/kithly-ws-go/internal/core/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load: %v", err)
	}

	addr := fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort)
	var redisOpt asynq.RedisClientOpt
	if cfg.RedisPassword != "" {
		redisOpt = asynq.RedisClientOpt{Addr: addr, Password: cfg.RedisPassword}
	} else {
		redisOpt = asynq.RedisClientOpt{Addr: addr}
	}

	h := asynqmon.New(asynqmon.Options{
		RootPath:     "/",
		RedisConnOpt: redisOpt,
	})
	defer h.Close()

	mux := http.NewServeMux()
	mux.Handle("/", h)

	listenAddr := fmt.Sprintf(":%d", cfg.DashboardPort)
	log.Printf("asynqmon dashboard listening on %s", listenAddr)
	if err := http.ListenAndServe(listenAddr, mux); err != nil {
		log.Fatalf("dashboard server: %v", err)
	}
}
