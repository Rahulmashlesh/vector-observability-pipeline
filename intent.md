# Intent

Pre-work summary for approval. See `DESIGN.md` for full rationale,
`ARCHITECTURE.md` for component detail, `AGENT.md` for operating notes.

## What

A 3-tier Vector log pipeline (agent → centralizer → egress) shipping logs
to self-hosted Loki, queryable via the existing Grafana instance. Built as
both a homelab operational improvement and a public portfolio piece.

## Scope approved

- No multi-tenant console, no per-tenant routing (single operator, single
  cluster — see `DESIGN.md`).
- Destination: self-hosted Loki (Datadog's free tier does not cover logs —
  confirmed, no indefinite-free SaaS option exists).
- 3 tiers kept, justified by separation of concerns, not node count.
- CRD/operator deferred to Stage 4 — built after Stages 1-3 are proven
  with hand-edited config, not designed upfront.

## Repo split

- `vector-observability-pipeline` (this repo, public): pipeline source,
  sanitized configs, docs.
- `endurance-platform` (private): real Argo CD Applications, real cluster
  values.

## Status

Scaffolded 2026-10-02. Stage 1 (agent tier) not yet started. Loki's memory
footprint on 8GB nodes not yet checked.
