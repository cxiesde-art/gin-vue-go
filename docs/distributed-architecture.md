# Distributed Architecture

The deployment has three processes that share a SQLite volume in the default Compose example. A production deployment can move the database to a managed SQL service without changing the process boundaries.

```text
Vue client
    |
    v
Gin API :8080 ---- gRPC/JSON ----> Identity service :9091
    |
    +---- gRPC/JSON ----> Content service :9090
    |
    +---- audit_events + outbox_events ----> Outbox worker
```

The identity service owns credential verification and token issuance. The API owns admin writes, moderation, audit records, revisions, and outbox records. The content service owns public post, search, category, and tag reads. The API falls back to local implementations when the corresponding gRPC address is empty, which makes unit tests and single-process development deterministic.

Each HTTP request receives `X-Request-ID` and `X-Trace-ID` values. The gRPC client forwards both values as metadata, and the content server logs them with the RPC method, status code, and duration. The API exposes Prometheus-compatible counters at `/metrics` and dependency readiness at `/readyz`.

Public reads retry transient gRPC failures with bounded exponential backoff. A circuit breaker opens after repeated failures, allows one probe after the recovery window, and closes after a successful response. Permanent errors are returned immediately. Admin writes are recorded as an audit event and an outbox event in one database transaction; the worker marks an event published only after its publisher succeeds.

## Local smoke test

```bash
docker compose up --build
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
curl http://localhost:8080/metrics
```

The default outbox publisher writes successful deliveries to the worker log. Replace `outbox.LogPublisher` with a broker or webhook publisher when integrating another service.
