# Lessons from a real Compose → k3s migration

Source: the Kithly API migration (single home server, Cloudflare Tunnel in front, Compose →
k3s 1.36 + Helm via GitHub Actions, managed Postgres → in-cluster Postgres), cut over 2026-10-11
with ~12 s downtime. Incidents are marked **INCIDENT**. Cite as `L<n>`; IDs are stable — append,
never renumber.

## Host and k3s install

- **L1 Pin versions.** Pin k3s (`v1.36.5+k3s1`) and fetch `install.sh` from the *same tag* on
  GitHub, not `get.k3s.io` (a moving target). Pin Helm and verify its sha256.
- **L2 Idempotent bootstrap.** Write `/etc/rancher/k3s/config.yaml` from the script; restart k3s
  only when the config or version changed. Safe to re-run.
- **L3 Disk.** If `/` is tight (it was 84% full, shared with Docker), set `data-dir` to the big
  disk and bind-mount `/var/lib/kubelet` there too (fstab entry) — emptyDir and images fill
  `/` otherwise.
- **L4 DNS.** Give k3s its own `resolv-conf` with plain IPv4 upstreams. The host file had IPv6
  link-local servers (`fe80::…%2`) and >3 entries; CoreDNS can't use zone-scoped addresses and
  kubelet keeps only three.
- **L5 Firewall.** With ufw, allow `10.42.0.0/16` (pods) and `10.43.0.0/16` (services). Without
  it, pod → CoreDNS traffic is dropped and DNS fails *silently*.
- **L6 Node IP.** Pin `node-ip` when the host has several NICs on one subnet (wired + wifi).
- **L7 Swap.** If the host keeps swap for other workloads, `kubelet-arg: fail-swap-on=false`.
- **L8 Kubeconfig access. INCIDENT (contributed to L10's outage).** k3s's bundled `kubectl`
  reads `/etc/rancher/k3s/k3s.yaml`, root-only by default. A non-root bootstrap step silently
  failed, so the step that would have disabled ServiceLB never ran. Set
  `write-kubeconfig-mode: "0640"` and `write-kubeconfig-group`.
- **L9 Kernel.** `overlay`, `br_netfilter`, `ip_forward`, `bridge-nf-call-iptables` persisted
  in `modules-load.d` / `sysctl.d`.

## Running alongside Compose, and cutover

- **L10 ServiceLB hijacks ports. INCIDENT: ~8 min public outage.** ServiceLB's hostPorts are
  iptables DNAT, not socket binds — they do not fail when Docker holds `:80`; they *take* the
  traffic immediately. Keep ServiceLB **off from first boot** with a registration-time node
  label (`svccontroller.k3s.cattle.io/enablelb=false`) in a config drop-in
  (`/etc/rancher/k3s/config.yaml.d/50-servicelb.yaml`). k3s re-applies `node-label` on every
  start, so the on/off switch script must update **both** the drop-in and the live label, and
  must refuse `on` while Docker still listens on the ports. → `assets/servicelb.sh`
- **L11 Same ports, unchanged front door.** Let k8s take over the exact host ports Compose used.
  The tunnel / reverse-proxy config then never changes, cutover is one switch, and rollback is
  the reverse switch. Stop (don't remove) Compose containers at cutover so rollback is
  `docker start`.
- **L12 Verify before cutover without touching live ports.** (a) Hit Traefik's auto-assigned
  NodePort with a `Host:` header. (b) Prove `localhost:<hostPort>` works (the path the tunnel
  uses) with a throwaway hostPort pod on a spare port (`:18080`), not by enabling ServiceLB.
- **L13 One deployer at a time.** Disable the Compose deploy workflow before cutover — a
  Compose redeploy would fight k8s for the ports. During overlap, put both workflows in the same
  concurrency group.
- **L14 Queues and schedulers.** Give k8s its own Redis so the two stacks don't share queues;
  drain the Compose queue before `docker compose down` (queued jobs are not migrated). Check for
  cron/scheduled jobs that would double-run while both stacks are up.

## Ingress

- **L15 Use what k3s ships.** Bundled Traefik; switch on its Gateway API provider with a
  `HelmChartConfig` dropped in `<data-dir>/server/manifests/`. Gateway API CRDs come with k3s's
  `traefik-crd` chart — never install a second copy. The app chart owns its `Gateway` and
  `HTTPRoute`s.
- **L16 TLS.** Behind a Cloudflare Tunnel, TLS ends at Cloudflare; `:443` gets bound but nothing
  needs to route to it. Public DNS doesn't change (hostnames CNAME to the tunnel).
- **L17 CNI.** Default Flannel (VXLAN) is fine single-node; k3s's embedded NetworkPolicy
  controller is on but does nothing until you write policies.

## Images, chart and CI

- **L18 Build in CI, one image per binary.** Never build on the server. Push to a registry
  (GHCR) tagged by commit SHA; cluster pulls with a read-only token as an imagePullSecret.
  `provenance: false` so each push is one package version, then keep the newest N (20) for
  `helm rollback`.
- **L19 `.dockerignore`.** Add one before the first CI build so a stray `.env` is never baked in.
- **L20 Secrets never in values or argv.** Write them to a `umask 077` file on the runner, stream
  over SSH stdin into `kubectl create secret … --dry-run=client -o yaml | kubectl apply -f -`.
  Not on a command line, not in helm values (they'd land in release history).
- **L21 Roll pods on secret change.** Put a short sha256 of the env file in a pod annotation via
  `--set-string envChecksum=…`.
- **L22 Generated passwords.** Generate on first install and reuse via `lookup`; annotate
  `helm.sh/resource-policy: keep`.
- **L23 local-path PVCs bind on first consumer. INCIDENT: first deploy timed out.** local-path
  is `WaitForFirstConsumer`; a PVC used only by a CronJob stays `Pending` and
  `helm upgrade --wait` hangs. Use a `hostPath` directory for backup output.
- **L24 Database move.** Behind a flag (`DB_IN_CLUSTER`) that only overrides the connection URL,
  so rollback is flipping it back. Dump over a **session-mode** connection (direct, or the
  provider's session pooler), never a transaction pooler (PgBouncer `pgbouncer=true`) — pg_dump
  fails through it. Refuse a non-empty target
  unless forced; verify per-table row counts; stop writers during the final copy; get off-machine
  backups (dedicated private bucket) from day one; keep the old DB paused, not deleted.
- **L25 Deploy safety.** `helm upgrade --install --rollback-on-failure --timeout 10m`; on
  failure print events and the last 30 log lines of every pod; always print `helm history`.
- **L26 Validate rendered output in PR CI.** `helm lint` + `helm template | kubeconform -strict`
  with the CRDs-catalog schema location (Gateway API kinds), once per feature-flag variant.
  This caught a duplicated label that made every Deployment invalid.
- **L27 Probes and resources** on every container; workers without HTTP get exec/none but still
  get resources.
- **L28 Pre-flight in deploy.** The deploy job checks the cluster is bootstrapped (helm present,
  namespace exists) and fails with a pointer to the bootstrap workflow instead of a cryptic error.
- **L29 App connection settings.** If the app was tuned for a pooler (PgBouncer → pgx simple
  protocol, `connection_limit=1`), check behaviour against direct Postgres and size
  `max_connections` for (processes × pool size).
- **L30 Every deploy workflow, every branch.** Inventory all workflows that deploy, not just the
  one on `main`: branch-triggered Compose deploys to other hosts were still in the repo after
  the k3s cutover, one push away from starting a second stack. Check the trigger branch still exists (`git ls-remote --heads`);
  dead ones should be disabled (`gh workflow disable`) or deleted — ask which the human prefers.
