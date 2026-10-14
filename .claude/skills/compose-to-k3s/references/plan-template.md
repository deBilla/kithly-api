# <Project> — Compose → k3s migration plan

**Status:** DRAFT — awaiting Gate B review
**Rev 1 (YYYY-MM-DD):** initial plan.

## Approvals

| Gate | Date | Decision | Notes |
|---|---|---|---|
| A — Facts | | | |
| B — Plan review | | | |
| C — Code review | | | |
| D — Host changes | | | |
| E — Cutover | | | |
| F — Data migration | | | |
| G — Decommission | | | |

## 1. Current state (from discovery)

- Host: OS, CPU/RAM, disks, NICs, firewall, front door (tunnel/proxy) and its targets.
- Services: one row each.

| Compose service | Image/build | Host ports | State | Exposure |
|---|---|---|---|---|

- Deploy pipeline today.
- **ASSUMED:** anything not confirmed at Gate A.

## 2. Target state

| Compose service | k8s object(s) | Exposure | Probe | Resources | Notes |
|---|---|---|---|---|---|

- Ingress: bundled Traefik + Gateway API (L15); hostnames and ports that move.
- Images/registry/tags (L18). Secrets: names, where they live, how they sync (L20, L21).
- State: per volume/DB — fresh, copied, or migrated later (Gate F).

## 3. Host changes (bootstrap)

What the bootstrap does on this host, mapped to lessons (L1–L9, L10, L15). Uninstall path.

## 4. Lessons

| Lesson | Applied how | Or skipped because |
|---|---|---|

## 5. Rollout

1. PR with chart + scripts + CI (Gate C).
2. Bootstrap (Gate D).
3. Parallel run; verification commands (L12).
4. Cutover (Gate E) — exact commands:
   ```sh
   ```
   Expected downtime:
5. Data migration (Gate F), if any — exact commands and verification.
6. Soak period: <N days>, then decommission (Gate G).

## 6. Rollback

For each step above: trigger, exact commands, data-loss window.

## 7. Risks and open questions

| ID | Risk / question | Likelihood | Impact | Mitigation |
|---|---|---|---|---|

## 8. Outcome (filled in at the end)
