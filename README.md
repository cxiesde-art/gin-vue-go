# Gin Vue Blog Go

`gin-vue-blog-go` is a distributed Go backend for a Vue blog frontend. It is inspired by the backend responsibilities in [szluyu99/gin-vue-blog](https://github.com/szluyu99/gin-vue-blog), while keeping the implementation independent and focused on a clean API contract.

The service provides public post browsing, Markdown-friendly post content, moderation-ready comments, admin CRUD, JWT authentication, role checks, pagination, SQLite persistence, and a health endpoint. Public content reads can run in a separate gRPC content service, while the Gin API handles authentication and writes. Structured JSON logs carry request and trace IDs across HTTP and gRPC. GORM migrations run automatically on startup, so a new checkout can be started without a separate database setup step.

## Run locally

Requirements: Go 1.23 or newer.

```bash
go mod download
go run ./cmd/server
```

The server listens on `http://localhost:8080`. The first start creates `blog.db` and seeds an administrator using:

```text
BLOG_ADMIN_EMAIL=admin@example.com
BLOG_ADMIN_PASSWORD=admin123
```

Set `BLOG_DATABASE_URL`, `BLOG_ADDRESS`, and `BLOG_JWT_SECRET` for a deployment. Never use the development JWT secret or administrator password in production.

`BLOG_MAX_BODY_BYTES` limits JSON request bodies and defaults to 1 MiB. The API also emits browser security headers on every response.

`BLOG_RATE_LIMIT_REQUESTS` and `BLOG_RATE_LIMIT_WINDOW` configure the gateway's per-client request window. In a multi-instance deployment, place a shared rate limiter at the edge as well; this in-process limiter protects each instance independently.

## Distributed mode

Run the two-service stack with Docker Compose:

```bash
docker compose up --build
```

The `content` service listens on gRPC `:9090` and owns public content reads. The `worker` process publishes pending outbox events. The Gin API listens on HTTP `:8080`; set `BLOG_CONTENT_GRPC_ADDRESS` to enable the remote reader. When the variable is empty, the API uses its local service implementation, which keeps local development simple. `X-Request-ID` and `X-Trace-ID` are propagated through gRPC metadata and included in structured request logs. Use `/healthz` for process health and `/readyz` for dependency readiness.

The API retries transient gRPC `Unavailable`, `DeadlineExceeded`, and `ResourceExhausted` responses with bounded exponential backoff. Set `BLOG_CONTENT_GRPC_RETRY_ATTEMPTS` and `BLOG_CONTENT_GRPC_RETRY_DELAY` (for example `100ms`) to tune the policy. Requests stop immediately on permanent errors or when the caller context is canceled.

The content client also uses a circuit breaker after repeated failures. It allows one probe after a five-second recovery window and closes again after a successful response.

Admin mutations also create durable audit and outbox records. `GET /api/v1/admin/audit-events` exposes the audit trail, while `GET /api/v1/admin/outbox-events` exposes unpublished events for a publisher worker.

Admin post writes retain immutable revisions. Use `GET /api/v1/admin/posts/:id/revisions` to inspect the newest version first.

## API examples

Log in to receive a bearer token:

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"admin123"}'
```

Create a published post with the returned token:

```bash
curl -X POST http://localhost:8080/api/v1/admin/posts \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Hello Vue","content":"# Welcome","status":"published"}'
```

Public clients use `GET /api/v1/posts`, `GET /api/v1/search?q=go`, `GET /api/v1/categories`, `GET /api/v1/tags`, `GET /api/v1/posts/:slug`, `GET /api/v1/posts/:slug/comments`, and `POST /api/v1/posts/:slug/comments`. Search requires at least two characters and only returns published posts. Admin clients use the `/api/v1/admin` routes to manage drafts, categories, tags, and approved comments.

Post slugs preserve letters and numbers from non-English titles. Public post detail requests increment the view counter; search, comment submission, and comment listing do not count as reads. Passwords use a random per-password Argon2id salt and never appear in API responses.

## Development

```bash
make fmt
make vet
make test
make build-content
```

The CI workflow also runs the test suite with the race detector and writes a coverage profile. The project is intentionally backend-only; a Vue 3 client can consume the versioned JSON API without coupling to the persistence layer.

## License

MIT
