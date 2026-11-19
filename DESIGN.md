# Design

## Problem

Running Kubernetes workloads across multiple namespaces produces logs that
disappear the moment a pod is evicted or restarted — `kubectl logs` only
reaches whatever the container runtime still retains locally. There is no
durable, queryable, cross-namespace view of what happened on the cluster.

## Goal

A log pipeline that collects from every node, centralizes and enriches in
one place, and ships to a durable, queryable backend — correlatable with
existing cluster metrics in the same dashboard.

## Scope decisions

### No multi-tenant admin console

A production version of this pattern typically includes a self-service
console where customers without cluster access choose log level, type, and
destination per tenant. That requirement is dropped here: there is one
operator with full cluster access, so a console would be a strictly worse
interface than editing a CRD spec or ConfigMap directly. Any UI layer here
would be solving a problem that does not exist at this scale.

### No per-tenant destination routing

One cluster, one (or two) destinations. Routing logic that exists to let
different customers pick different destinations has no counterpart here.

### Destination: self-hosted Loki, not a SaaS

Datadog's free tier was evaluated and rejected: it covers Infrastructure
Monitoring only (5 hosts, 1-day metric retention), with no free tier for
log ingestion — logs are billed from the first byte with no indefinite
free allowance. A self-hosted Loki instance costs nothing beyond cluster
resources already available, and fits the same "pluggable sink" role a
SaaS destination would have occupied, without a recurring cost or usage
cap. An existing Grafana instance (deployed via kube-prometheus-stack)
can add Loki as a datasource with no new UI.

### Three tiers: justified by separation of concerns, not node count

| Tier | Responsibility | Why it's separate |
|---|---|---|
| Agent (DaemonSet) | Read logs from the local node | Must run per-node; this is true at any node count |
| Centralizer (Deployment) | Single collection point, cluster-wide enrichment | Decouples "collect from every agent" from "talk to the destination"; enrichment logic lives once, not duplicated per agent |
| Egress (Deployment) | Destination-specific sink logic | Keeps destination auth/retry/batching swappable without touching collection logic |

A production deployment of this pattern also uses the centralizer tier to
fan in from multiple clusters; a single-cluster deployment loses that
specific justification but keeps the separation-of-concerns one. Collapsing
agent and centralizer into one tier was considered and rejected: it would
require enrichment logic and destination connectivity to be duplicated into
every per-node agent, instead of living once.

### Operator deferred to a later stage, not cut

A declarative CRD + Kubebuilder operator is the mechanism that would let a
production deployment reconcile tenant-level config changes automatically.
That mechanism is kept — multi-tenancy is not required to justify it, since
the real value is declarative config + automatic reconciliation + reload
on change, independent of audience size. It is deferred rather than built
first: designing a CRD before the real config shape is known is higher-risk
than encoding a config that has already been validated by hand. The operator
is built once the hardcoded pipeline (agent → centralizer → egress → Loki)
is proven working.

## Alternatives considered

| Option | Rejected because |
|---|---|
| Datadog (SaaS) | No free tier for logs; indefinite free hosting was a hard requirement |
| Single combined agent+centralizer tier | Forces enrichment + destination logic into every per-node agent |
| CRD/operator built first | Unvalidated config shape; higher risk than encoding a hand-tested config later |
| Multi-tenant console | No second tenant exists; solves a problem this deployment doesn't have |

## Staged build plan

1. Agent tier — DaemonSet, dummy sink, verify node → centralizer hop.
2. Centralizer tier — Deployment, dummy sink, verify fan-in + enrichment.
3. Egress tier + Loki — real destination, Grafana datasource wired in.
4. Operator — CRD + Kubebuilder controller replacing hand-edited config.
