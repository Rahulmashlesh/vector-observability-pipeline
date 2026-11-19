# Agent notes

Operating notes for whoever (human or AI agent) picks this project up next.
See `DESIGN.md` for rationale, `ARCHITECTURE.md` for component detail.

## Current stage

**Stage 0 — scaffold only.** Repo structure created
(`vector/`, `k8s/`, `operator/` directories exist but are empty except for
`.gitkeep`). No Vector config, no k8s manifests, no operator code written
yet.

## Next step

Stage 1 — Agent tier: write the Vector DaemonSet config
(`kubernetes_logs` source, dummy console/file sink) and its k8s manifest.
Verify logs flow from a node into the agent pod before wiring a real
forward to the centralizer tier.

## Safe to change without asking

- Anything under `vector/`, `k8s/`, `operator/` while they remain empty
  scaffolds — this repo has no live deployment yet.
- Documentation (`README.md`, `DESIGN.md`, `ARCHITECTURE.md`, this file).

## Requires a decision before proceeding

- **Loki's memory footprint on an 8GB-RAM cluster node** has not been
  checked. This must happen before Stage 3 commits to deploying Loki —
  if it doesn't fit, the destination choice in `DESIGN.md` needs revisiting
  before, not after, deployment.
- **GitOps Application manifests** (three `Application` resources under
  `gitops/apps/log-streaming/` in the private `endurance-platform` repo)
  have not been scaffolded yet. Do this only once Stage 1 produces a real
  manifest to point at — don't scaffold Argo CD Applications pointing at
  nothing.
- **CRD shape for the Stage 4 operator** is deliberately undecided — see
  `DESIGN.md`'s "Operator deferred" section. Do not design or scaffold the
  CRD before Stages 1-3 are working; the whole point of deferring is to
  base the CRD on a validated config.

## Explicitly out of scope (do not add without a scope conversation)

- Multi-tenant admin console / self-service UI.
- Per-tenant destination routing.
- Any SaaS logging destination with a paid or capped free tier (Datadog
  was evaluated and rejected for this reason — see `DESIGN.md`).

## Repo split

- **This repo** (`vector-observability-pipeline`, public): pipeline source
  code, generic/sanitized configs, docs. No real hostnames, IPs, or
  cluster-specific resource limits belong here.
- **`endurance-platform`** (private, separate repo): the actual Argo CD
  Applications and values overlays pointing at the real cluster. Cluster
  topology, node names, and real resource tuning live there, not here.
