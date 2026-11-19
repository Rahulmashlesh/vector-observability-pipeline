# Architecture

## Overview

```
┌──────────────┐     ┌───────────────┐     ┌──────────────┐     ┌──────┐
│ Vector Agent  │────▶│ Vector         │────▶│ Vector       │────▶│ Loki │
│ (DaemonSet,   │     │ Centralizer    │     │ Egress       │     └──────┘
│  per node)    │     │ (Deployment)   │     │ (Deployment) │        │
└──────────────┘     └───────────────┘     └──────────────┘        ▼
                                                                 Grafana
                             ▲                                 (datasource)
                             │ reconciles (Stage 4)
                       ┌─────┴──────┐
                       │  Operator   │
                       │ (Kubebuilder)│
                       └────────────┘
```

## Components

### Agent tier

- **Kind**: DaemonSet, one pod per node.
- **Source**: `kubernetes_logs` — reads container logs from the local
  node's log directory.
- **Transform**: none (minimal passthrough); enrichment happens downstream.
- **Sink**: Vector-native protocol, forwards to the centralizer tier.
- **Namespace**: `logging` (shared with centralizer/egress).

### Centralizer tier

- **Kind**: Deployment, single replica (no HA requirement at this scale).
- **Source**: Vector-native protocol, receives from every agent.
- **Transform**: adds cluster/node identity labels; the single point where
  cross-cutting enrichment lives (future: per-namespace filtering here).
- **Sink**: Vector-native protocol, forwards to the egress tier.
- **Namespace**: `logging`, dedicated.

### Egress tier

- **Kind**: Deployment, single replica.
- **Source**: Vector-native protocol, receives from the centralizer.
- **Sink**: `loki` sink type, destination-specific auth/batching logic
  lives here only. A second sink (e.g. `aws_s3` pointed at a self-hosted
  MinIO) can be added later without touching agent or centralizer config.
- **Namespace**: `logging`.

### Destination: Loki

- Self-hosted, deployed into the cluster (resource footprint check pending
  before Stage 3 — see current stage in `AGENT.md`).
- Added as a datasource to the existing Grafana instance
  (kube-prometheus-stack, `monitoring` namespace) — no new UI, LogQL
  queries correlate with existing Prometheus panels.

### Operator (Stage 4)

- Kubebuilder-based, Go module under `operator/`.
- Owns a CRD (working name: `LogRoute`) that declares which Vector
  tier(s) a config change applies to.
- Reconciles CRD changes into the `ConfigMap`s mounted by the agent/
  centralizer/egress Deployments, and triggers a config reload (Vector
  reloads on SIGHUP or via its own reload API — exact mechanism TBD when
  this stage starts).
- Replaces hand-edited ConfigMaps from Stages 1-3; built only after they
  are proven working (see `DESIGN.md` for why).

## Deployment / GitOps

- This repo holds pipeline source (Vector configs, k8s manifests, operator
  code) — no cluster-specific values (real hostnames, IPs, resource limits
  tuned to a specific node's memory).
- Actual deployment lives in a separate private GitOps repo, as three
  Argo CD `Application` resources (one per tier) under
  `gitops/apps/log-streaming/{agent,centralizer,egress}/`, each pointing
  at manifests derived from this repo's `k8s/` directory plus a private
  values overlay (real resource limits, node affinity).
- Three separate Applications (not one) for independent per-tier sync/
  health visibility in the Argo UI.

## Repository layout

```
vector/{agent,centralizer,egress}/   Vector pipeline configs (sources/transforms/sinks)
k8s/                                 K8s manifests: DaemonSet, Deployments, Services, ConfigMaps
operator/                            Kubebuilder operator (Stage 4): CRD types, controller
DESIGN.md                            Why — requirements, alternatives, scope decisions
ARCHITECTURE.md                      What — this document
AGENT.md                             Operating notes: current stage, what's safe to change
```
