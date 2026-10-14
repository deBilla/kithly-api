---
name: compose-to-k3s
description: Migrate a Docker Compose deployment on a single host to k3s + Helm, deployed by CI, with a reversible zero-surprise cutover. Use when the user wants to move a Compose stack (or a Compose-based deploy workflow) onto k3s/Kubernetes on the same or a new server. Orchestrates low-cost specialist subagents and stops at human approval gates (plan review, host changes, cutover, data migration, decommission).
---

# Compose → k3s migration

You are the **orchestrator**. You run on the main model, own every decision and every
human interaction, and delegate bounded, low-judgement work to specialist subagents that
run on cheaper models. Subagents never talk to the human and never change a server.

Paths below are relative to this skill's directory. Read before starting:
- `references/lessons.md` — what went wrong (and right) last time. Cite lessons by ID (`L7`).
- `references/handoffs.md` — the only formats subagents may return, and what you may send them.
- `references/plan-template.md` — the plan document the human reviews at Gate B.
- `assets/` — parameterised `bootstrap.sh`, `servicelb.sh`, and CI/chart snippets to adapt.

## Agents

| Agent | Model | Does | Never does |
|---|---|---|---|
| `k3s-compose-inventory` | haiku | Reads compose files, Dockerfiles, env templates, deploy workflows → `INVENTORY` handoff | Write files, run SSH |
| `k3s-host-recon` | haiku | Runs a fixed list of **read-only** commands on the host → `HOST_FACTS` handoff | Any command not on its allowlist |
| `k3s-chart-author` | sonnet | Writes chart, bootstrap, CI from the **approved plan file** → `BUILD` handoff | Touch the host, commit, push |
| `k3s-manifest-validator` | haiku | `helm lint`, `kubeconform`, lesson-checklist audit → `VALIDATION` handoff | Fix things (it reports; author fixes) |

Launch independent agents in one message so they run in parallel (inventory + recon).

## Context discipline (handover in both directions)

**To a subagent** send only: the skill directory's absolute path (the directory this SKILL.md
is in — agent files refer to it as `<skill-dir>`), repo path, host alias, the file paths it needs (e.g. the approved
plan), the lesson IDs relevant to its task, and — on a fix round — the specific findings to fix.
Never paste the conversation, other agents' full handoffs, or file contents it can read itself.

**From a subagent** accept only a handoff block in the format from `references/handoffs.md`.
If an agent returns prose, file dumps, or a transcript, do not forward it — re-ask once with
"Return only the `<TYPE>` handoff block", then extract what you need yourself.

You keep the handoffs; the human sees your synthesis, not raw handoffs.

## Approval gates

A gate is a hard stop. At each one, call `AskUserQuestion` with explicit options (first option =
proceed, then "Request changes", then "Stop"). Rules:

- **Auto mode, prior approvals and silence are not approval.** Each gate needs its own answer.
- Approval covers exactly what was shown. If the plan or diff changes afterwards, re-ask.
- Record each answer in the plan doc's *Approvals* table (gate, date, decision, notes).
- Present the material to approve **before** asking: what will happen, what can go wrong,
  how to undo it. For remote-mutating gates, show the exact commands and the rollback commands.

| Gate | When | Human approves |
|---|---|---|
| **A — Facts** | After discovery | Inventory and host facts are right; answers to open questions |
| **B — Plan review** | After you write the plan | The migration plan document (required, always) |
| **C — Code review** | After build + validation pass | The diff (chart, scripts, CI) before commit / PR |
| **D — Host changes** | Before bootstrap runs on the server | Installing k3s alongside the live stack |
| **E — Cutover** | Before k8s takes the ports | Downtime window + rollback path |
| **F — Data migration** | Before moving a database | Write freeze, copy, verification, flag flip |
| **G — Decommission** | After a soak period | Removing Compose containers/volumes, old workflow, old DB |

## Workflow

### 0. Scope
Establish: repo path, host SSH alias/user, how traffic reaches the host today (tunnel, reverse
proxy, direct ports), CI provider, registry, and whether a database moves too. Ask in one
`AskUserQuestion` for whatever the repo can't tell you.

### 1. Discovery (parallel, cheap)
Launch `k3s-compose-inventory` and `k3s-host-recon` together. Send each only the paths/host
and the handoff type to return. Check each handoff: status, every fact sourced, open questions.
If one is `blocked`, resolve the blocker (often a question for the human) before going on.

### Gate A
Summarise both handoffs in ≤15 lines: services, ports and who uses them, state (volumes, DBs,
queues), host risks (disk, DNS, firewall, NICs, swap). Ask the open questions. Don't plan on
unconfirmed assumptions; mark any you must make as **ASSUMED** in the plan.

### 2. Plan (you, not a subagent)
Write `docs/K3S_MIGRATION_PLAN.md` in the target repo from `references/plan-template.md`. Map
every Compose service to a k8s object; apply every relevant lesson and say which (`L10`), and
mark any lesson you consciously skip with the reason. The cutover and rollback sections must
contain exact commands.

### Gate B — plan needs human review
Tell the human the plan path and its key decisions and risks, then ask. On "Request changes",
revise and re-ask. Do not start Phase 3 until the answer is an approval of the current version.

### 3. Build → validate loop
1. `k3s-chart-author` gets: repo path, plan path, lesson IDs, `assets/` path. Returns `BUILD`.
2. `k3s-manifest-validator` gets: repo path, chart dir, the `BUILD` file list, plan path.
   Returns `VALIDATION`.
3. On `fail`, send the author **only** the failing findings. Max 2 fix rounds; after that,
   stop and bring the remaining findings to the human.

### Gate C
Show `git diff --stat`, the validator verdict, and any deviation from the plan the author
reported. On approval, branch, commit and open a PR (per the repo's conventions).

### 4. Bootstrap (remote, you run it)
Gate D first: show the bootstrap's actions, confirm it leaves the live stack's ports alone
(ServiceLB off from first boot — `L10`), and how to uninstall (`/usr/local/bin/k3s-uninstall.sh`).
Then run it (via the CI workflow if one exists, else SSH). Read its output yourself.

### 5. Parallel run
Deploy the chart while Compose still serves traffic. Verify through Traefik's NodePort with a
`Host:` header and the hostPort check (`L12`). `k3s-manifest-validator` may run its read-only
cluster checks here. Fix-forward through Phase 3 if needed (new diffs go back through Gate C).

### Gate E — cutover
Present: exact steps, expected downtime, the drain step for queues (`L14`), and the rollback
commands. After approval, execute step by step, checking each step's result before the next.
Stop and report on the first unexpected result; don't improvise around it.

### Gate F — data migration (if any)
Write freeze, copy, row-count verification, flag flip, backup, keep source paused (`L24`).

### Gate G — decommission
After the soak period the human chose: remove Compose containers, the old deploy workflow, and
(later) the old database. Update docs. Close the plan with an outcome section.

## Reporting

After each phase give the human 3–6 lines: what ran, result, what's next, and which gate is
coming. Put detail in the plan doc, not chat. Never claim a step succeeded without having read
its output.
