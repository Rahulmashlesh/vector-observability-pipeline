# vector-observability-pipeline

A 3-tier [Vector](https://vector.dev) log pipeline for Kubernetes — per-node
agent DaemonSet → cluster centralizer → egress — with a Kubebuilder operator
for dynamic reconciliation. Built as a homelab-scoped recreation of a
production multi-cluster log-streaming architecture.

See [`intent.md`](intent.md) for full scope, design rationale, and the
staged build plan.

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐     ┌──────┐
│ Vector Agent │────▶│ Vector        │────▶│ Vector      │────▶│ Loki │
│ (DaemonSet,  │     │ Centralizer   │     │ Egress      │     └──────┘
│  per node)   │     │ (Deployment)  │     │ (Deployment)│
└─────────────┘     └──────────────┘     └─────────────┘
                            ▲
                            │ reconciles
                      ┌─────┴──────┐
                      │  Operator   │  (Stage 4)
                      │ (Kubebuilder)│
                      └────────────┘
```

- **Agent**: reads container logs per-node, forwards to the centralizer.
- **Centralizer**: single collection point, cluster-wide enrichment.
- **Egress**: destination-specific sink logic (Loki today; S3/MinIO possible
  later).
- **Operator** (Stage 4): declarative config via CRD, replacing hand-edited
  ConfigMaps once the hardcoded pipeline is proven.

## New Relic sink (egress)

The egress tier also ships logs to New Relic over OTLP/HTTP protobuf
(`https://otlp.nr-data.net:4318/v1/logs`, US region), in addition to Loki.
The license key is read from the `newrelic-license` Secret, which is mounted
into the egress pod and read through Vector's `directory` secret backend
(`SECRET[newrelic.license-key]`). The Secret is not in git; create it by hand
before syncing:

```sh
kubectl -n logging create secret generic newrelic-license \
  --from-literal=license-key=<NEW_RELIC_LICENSE_KEY>
```

Without the Secret the egress pod stays in `ContainerCreating`.

## Status

Early scaffold — see [`intent.md`](intent.md) for current stage.

## Why

Written up for both operational use (durable, queryable logs across a
homelab Kubernetes cluster) and as a portfolio piece demonstrating the same
systems-design pattern as a production observability pipeline, scoped down
to single-tenant/single-cluster.
