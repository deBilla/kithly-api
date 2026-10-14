# Handoff formats

Every subagent returns **exactly one** fenced `yaml` block of its type and nothing else — no
preamble, no closing remarks. The orchestrator keeps it; the human never sees it raw.

## Rules for every handoff

- **≤ 80 lines — a hard limit.** Count before returning. To fit: omit any field whose value is
  `none`, empty or a default (no `restart: none`, no `volumes: none`); put a `src` on the same
  line as its fact; one line per list item; never add fields the schema doesn't have.
- **Only open questions that the repo can't answer.** Before raising one, check the obvious
  nearby lines (an "is X still in the docs?" question needs the surrounding paragraph read).
- **No file contents, logs or transcripts.** Point to them: `path:line`, or the command that
  shows them. Quote at most one line of an error.
- **Every fact has a `src`** (`path:line`, or the exact read-only command). Unsourced = guess;
  put guesses under `assumptions`.
- **`status`**: `complete` | `partial` (some items missing — list them) | `blocked` (can't
  continue without a human answer or access — say which).
- **`open_questions`** are for the human, phrased so they can be answered in one line. Subagents
  never ask the human directly.
- Secret **values** never appear. Names of secrets/env vars are fine.

## Common envelope

```yaml
handoff: <TYPE>
agent: <agent name>
status: complete | partial | blocked
summary: <≤3 lines>
# …type-specific sections…
assumptions: [ "<what was assumed and why>" ]
open_questions: [ "<one-line question for the human>" ]
```

## INVENTORY — `k3s-compose-inventory`

```yaml
handoff: INVENTORY
agent: k3s-compose-inventory
status: complete
summary: 5 services from docker-compose.yml; 1 public port (80); state in redis volume only.
services:
  - name: api
    src: docker-compose.yml:12
    image_or_build: build ./ (Dockerfile, ARG BINARY=api)
    command: <only if overridden>
    ports: ["80:3001"]            # host:container
    exposure: public | lan | internal
    env_from: [".env"]             # names of env files / vars, never values
    env_vars: [DATABASE_URL, REDIS_HOST]
    volumes: [{name: redis-data, target: /data, kind: named|bind, src: docker-compose.yml:40}]
    depends_on: [redis]
    healthcheck: "GET /health" | none
    restart: unless-stopped
    resources: {mem_limit: 512m} | none
    k8s_mapping_hint: Deployment + Service + HTTPRoute
stateful: [{service: redis, data: AOF on volume redis-data, migrate: fresh|copy}]
scheduled_jobs: [{what: "", src: ""}]       # cron, repeatable queue jobs — double-run risk (L14)
deploy_pipeline: {file: .github/workflows/deploy.yml, trigger: push main, how: "ssh + docker compose up"}
front_door: {kind: cloudflare-tunnel|nginx|direct, src: "", ports_used: [80, 3002]}
secrets_required: [GEMINI_API_KEY, AWS_SECRET_ACCESS_KEY]
dockerignore: present | missing
```

## HOST_FACTS — `k3s-host-recon`

```yaml
handoff: HOST_FACTS
agent: k3s-host-recon
status: complete
summary: Ubuntu 24.04, 16G RAM, / 84% full, /home 320G free; ufw active; resolv.conf has IPv6 link-local.
os: {distro: "", kernel: "", arch: amd64}
resources: {cpu: 8, mem_gb: 16, swap_gb: 4}
disks: [{mount: /, size: 100G, used_pct: 84}, {mount: /home, free: 320G}]
network:
  nics: [{name: enp3s0, ip: 10.0.0.13/24}, {name: wlp2s0, ip: 10.0.0.27/24}]
  resolv_conf: {nameservers: ["10.0.0.1", "fe80::1%2"], issue: "IPv6 link-local (L4)"}
  firewall: {tool: ufw, active: true, allows_pod_cidrs: false}
listening_ports: [{port: 80, owner: docker-proxy}, {port: 3002, owner: docker-proxy}]
docker: {version: "", compose_projects: [deployment], running_containers: 6}
existing_k8s: none | {kind: k3s, version: ""}
front_door: {cloudflared: running, config: /etc/cloudflared/config.yml, targets: ["localhost:80"]}
sudo: passwordless | password | none
risks: [{id: R1, lesson: L3, detail: "/ 84% full — put data-dir on /home"}]
```

## BUILD — `k3s-chart-author`

```yaml
handoff: BUILD
agent: k3s-chart-author
status: complete
summary: Chart for 5 services + redis; bootstrap + servicelb scripts; deploy and bootstrap workflows.
files:
  - {path: deploy/helm/app/templates/services.yaml, purpose: "Deployments+Services from values.services"}
plan_coverage: [{plan_item: "P3 api → Deployment+HTTPRoute", status: done}]
lessons_applied: [L10, L18, L20, L21, L23]
deviations_from_plan: [{plan_item: "", what: "", why: ""}]   # empty if none
self_checks: {helm_lint: pass, helm_template: pass}          # if the tools exist locally
todo_for_human: ["Create GHCR_PULL_TOKEN secret (read:packages)"]
```

## VALIDATION — `k3s-manifest-validator`

```yaml
handoff: VALIDATION
agent: k3s-manifest-validator
status: complete
verdict: pass | fail
summary: lint+kubeconform clean on 2 variants; 1 blocking finding.
tool_results:
  - {check: "helm lint", result: pass}
  - {check: "kubeconform -strict (default values)", result: pass}
  - {check: "kubeconform -strict (database.inCluster=true)", result: fail, first_error: "<one line>"}
findings:
  - id: V1
    severity: blocking | warning
    lesson: L23
    where: deploy/helm/app/templates/postgres.yaml:150
    problem: "Backup PVC only consumed by CronJob — stays Pending, helm --wait hangs"
    fix_hint: "hostPath dir for backups"
checklist: {L10: ok, L20: ok, L21: missing, L23: violated, L25: ok, L26: ok, L27: partial}
```

## What the orchestrator sends (inbound)

Keep it to a short brief, never the conversation:

```
Task: <one line>
Skill dir: <absolute path of the compose-to-k3s skill directory — written <skill-dir> in agent files>
Repo: <path>          Host: <ssh alias or "n/a">
Read: <file paths the agent needs, e.g. docs/K3S_MIGRATION_PLAN.md>
Lessons: <IDs relevant to this task> (full text: <skill-dir>/references/lessons.md)
Fix only: <V1, V3 — on fix rounds, the findings verbatim from VALIDATION>
Return: the <TYPE> handoff block from <skill-dir>/references/handoffs.md
```
