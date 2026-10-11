# Kubernetes (k3s) deployment

The API, queue workers and transcoder run on a single-node **k3s** cluster, deployed from
GitHub Actions with **Helm**. Public traffic reaches the host through a Cloudflare Tunnel and
enters the cluster through the **Gateway API** (k3s's bundled Traefik).

```
User → Cloudflare (TLS) → cloudflared on the host → localhost:80
     → ServiceLB (hostPort) → Traefik → Gateway → HTTPRoute → Service → api pod
```

## Pieces

| Path | What it is |
|---|---|
| `deploy/k3s/bootstrap.sh` | Installs/upgrades k3s + Helm on the host. Idempotent. |
| `deploy/k3s/servicelb.sh` | Switches ServiceLB (k8s ownership of host ports) on/off. |
| `.github/workflows/k3s-bootstrap.yml` | Runs the bootstrap over SSH. Manual; type `bootstrap` to confirm. |
| `deploy/helm/kithly/` | Chart: api, worker ×4, transcoder, dashboard, playground, Redis, Postgres + nightly backup CronJob, Gateway + HTTPRoute. |
| `.github/workflows/deploy-k8s.yml` | PRs: tests + chart validation. Push to `main` (not docs-only) or dispatch: build → GHCR → `helm upgrade`. |

## Pipeline

1. `test`: `go vet`, `go test -race ./...`
2. `chart`: `helm lint`, `helm template | kubeconform -strict` (including Gateway API schemas)
3. `build`: one image per `cmd/<binary>`, `ghcr.io/<owner>/kithly-<binary>:<sha>`, built on GitHub
   runners with the job's `GITHUB_TOKEN`.
4. `deploy` (only when the `DEPLOY_HOST` variable is set):
   - GitHub secrets → `kithly-env` Secret and a `ghcr-pull` pull secret, streamed over SSH stdin,
     never on a command line or in Helm values.
   - `helm upgrade --install --rollback-on-failure`: a release whose pods aren't ready in 10 min is
     rolled back automatically.
   - A checksum of the secret contents is a pod annotation, so rotating a secret rolls the pods.
5. `cleanup-images`: keeps the newest 20 versions of each image, so `helm rollback` can reach 19 back.

Roll back: `helm -n kithly history kithly`, then `helm -n kithly rollback kithly <revision>`.

## Setup

**Repository variables:** `DEPLOY_HOST` (`user@host`), `NODE_IP` (the host's LAN IP), `UPSTREAM_DNS`
(e.g. your router), optional `DB_IN_CLUSTER` (default `true`), optional `DB_BACKUP_S3_URI`.

**Secrets:**
- SSH: `SERVER_SSH_PRIVATE_KEY`, `KNOWN_HOSTS`, `SSH_CONFIG`. The workflows install `cloudflared`, so
  `SSH_CONFIG` can use `ProxyCommand cloudflared access ssh --hostname %h`.
- Registry: `GHCR_PULL_TOKEN`, a classic PAT with only `read:packages`.
- App: `GEMINI_API_KEY`, `AWS_REGION`, `AWS_S3_BUCKET`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`,
  `FIREBASE_SERVICE_ACCOUNT_BASE64`, optional `ADMIN_API_KEY`, `MAILGUN_API_KEY`, and `DATABASE_URL`
  (only when `DB_IN_CLUSTER=false`).

**Order:**
1. Run **Bootstrap k3s**. It installs k3s with ServiceLB **off**, enables Traefik's Gateway API
   provider, and checks that a hostPort is reachable on `localhost` (the path a host-local tunnel uses).
2. Run **Deploy to k3s**. Smoke-test through Traefik's NodePort before exposing anything:
   `curl -H 'Host: api.example.com' "http://<node-ip>:$(kubectl -n kube-system get svc traefik -o jsonpath='{.spec.ports[?(@.name=="web")].nodePort}')/health"`
3. Free host ports 80/3002/3003 if something else holds them, then `servicelb.sh on`.

## Host layout

- k3s data (images, PVCs) lives in `/home/k3s`. Pod scratch (`/var/lib/kubelet`) is bind-mounted from
  `/home/k3s-kubelet`. Both are kept off the root filesystem.
- **Ports.** ServiceLB binds the LoadBalancer Services on the host: Traefik on :80/:443, the
  dashboard on :3002, the playground on :3003. A tunnel on the host targets `localhost:<port>`.
- **Gateway API.** k3s's `traefik-crd` chart ships the Gateway API CRDs, so don't install a second
  copy. The bootstrap switches the provider on with a `HelmChartConfig` and turns off Traefik's default
  Gateway; this chart owns its own `Gateway` and `HTTPRoute`s. Add hostnames under `gateway.routes`.
- **DNS.** CoreDNS resolves in-cluster names (`kithly-postgres`, `kithly-redis`). k3s gets its own
  `resolv.conf` pointing at `UPSTREAM_DNS`.

### ServiceLB gotcha: hostPort is DNAT, not a socket bind

ServiceLB exposes ports as pod `hostPort`s, which are iptables DNAT rules. They do **not** wait for
another process (e.g. Docker) to release the port. The moment ServiceLB is on, it takes that port's
traffic. When migrating from Docker Compose on the same host, that silently diverts production traffic.

So ServiceLB starts **off**: a k3s config drop-in (`/etc/rancher/k3s/config.yaml.d/50-servicelb.yaml`)
registers the node with `svccontroller.k3s.cattle.io/enablelb=false`. k3s re-applies `node-label` on every
start, so the drop-in is the persistent half of the switch. `deploy/k3s/servicelb.sh on|off` updates
the drop-in and the live label together, and refuses `on` while Docker still listens on the ports.

```sh
ssh <user>@<host> 'bash -s' -- on  < deploy/k3s/servicelb.sh   # k8s takes the ports
ssh <user>@<host> 'bash -s' -- off < deploy/k3s/servicelb.sh   # k8s releases them
```

## Postgres

- A StatefulSet `kithly-postgres` with a 20Gi local-path PVC and `max_connections=200` (each Go
  process pools up to 25).
- The password is generated on first install into the secret `kithly-postgres`, reused on every
  upgrade (`lookup`), and kept on uninstall.
- With `DB_IN_CLUSTER=true` the chart overrides `DATABASE_URL` with a direct connection to
  `kithly-postgres`.
- **Backups:** CronJob `kithly-postgres-backup` runs nightly `pg_dump -Fc` into
  `/home/kithly-backups/postgres` on the host and keeps 14 days. Set `DB_BACKUP_S3_URI` to a dedicated
  private bucket for an off-machine copy; without it, a disk failure loses the data.
  - It's a hostPath rather than a PVC on purpose. local-path binds a PVC only when its first consumer
    runs, and a PVC used only by a CronJob stays Pending, which blocks `helm upgrade --wait`.
- Ad-hoc backup: `kubectl -n kithly create job --from=cronjob/kithly-postgres-backup manual-$(date +%s)`.
- Restore (destructive; stop the app first):
  `kubectl -n kithly exec -i kithly-postgres-0 -- pg_restore -U kithly -d kithly --clean --if-exists --no-owner < backup.dump`
