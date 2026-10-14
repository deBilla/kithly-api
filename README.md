# Kithly API

Backend for **Kithly**, a group video card app. An organizer creates a card, friends upload short
video messages, and the service compiles them into one shareable video with music, overlays and a
cover frame.

Written in Go and deployed to a single-node k3s cluster with Helm from GitHub Actions.

## Architecture

| Process | `cmd/` | Role |
|---|---|---|
| API | `api` | Gin HTTP API: cards, contributions, users, reference data |
| Worker | `worker` | asynq consumer: compiles card videos with ffmpeg |
| Transcoder | `transcoder` | asynq consumer: transcodes uploads to CMAF/HLS adaptive bitrate |
| Dashboard | `dashboard` | Queue inspector for the asynq queues (no built-in auth: put it behind your own) |
| Playground | `playground` | Small web UI for exercising the API |

**Stack:** Go · Gin · sqlc + pgx/v5 · PostgreSQL · Redis + asynq · S3 + CloudFront · Firebase Auth
and FCM · Gemini · ffmpeg.

**Layout** (clean architecture: handler → service → repository, all behind interfaces):

```
cmd/                 entry points; cmd/api/main.go wires all dependencies
internal/modules/    one package per domain: handler, service, repository, dto, entity
internal/middleware/ auth (Firebase ID token or guest token), errors, logging
internal/core/       config, db pool, redis, errors, response envelope, sqlc output
pkg/                 S3, Firebase, Gemini, ABR/HLS helpers
db/                  schema + sqlc queries
deploy/              Helm chart and k3s bootstrap
```

**Auth:** `Authorization: Bearer <Firebase ID token>`, or `X-Guest-Token` for guest accounts.
Reference-data writes require `X-Admin-Key`.

## Run locally

```sh
cp .env.example .env              # fill in your own values
docker compose up --build         # api on :80, dashboard :3002, playground :3003, redis
go test -race ./...
```

Configuration is environment-only (`internal/core/config/config.go`): `DATABASE_URL`, `REDIS_HOST`,
`AWS_*`, `FIREBASE_SERVICE_ACCOUNT_BASE64`, `GEMINI_API_KEY`, and optionally `ADMIN_API_KEY`.

## Deploy

See [docs/K8S_DEPLOYMENT.md](docs/K8S_DEPLOYMENT.md):
- the k3s bootstrap and Helm chart
- the GitHub Actions pipeline (test → build → GHCR → `helm upgrade` with automatic rollback)
- Gateway API routing, the in-cluster Postgres with nightly backups
- a ServiceLB/hostPort gotcha worth knowing before migrating from Docker Compose

### Claude Code skill: Compose → k3s

The migration that produced this setup is packaged as a [Claude Code](https://claude.com/claude-code)
skill in [`.claude/skills/compose-to-k3s/`](.claude/skills/compose-to-k3s/SKILL.md), with its
specialist subagents in [`.claude/agents/`](.claude/agents/). The main agent plans the migration
and handles every human approval: plan review, host changes, cutover, data migration and
decommission. Cheaper models (Haiku, plus Sonnet for the chart) do inventory, read-only host
recon, chart writing and validation, and hand back fixed-format summaries rather than their whole
context. [`lessons.md`](.claude/skills/compose-to-k3s/references/lessons.md) is worth reading on
its own.

It loads automatically when Claude Code runs in this repo. To use it on other projects, copy both
into your user config:

```sh
cp -R .claude/skills/compose-to-k3s ~/.claude/skills/
cp .claude/agents/k3s-*.md ~/.claude/agents/
```

## License

[MIT](LICENSE)
