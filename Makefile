.PHONY: run run-content run-identity run-comments run-profile run-analytics run-taxonomy run-search run-audit run-posts run-worker test coverage fmt vet build build-content build-identity build-comments build-profile build-analytics build-taxonomy build-search build-audit build-posts build-worker

run:
	go run ./cmd/server

run-content:
	go run ./cmd/content-server

run-identity:
	go run ./cmd/identity-server

run-comments:
	go run ./cmd/comment-server

run-profile:
	go run ./cmd/profile-server

run-analytics:
	go run ./cmd/analytics-server

run-taxonomy:
	go run ./cmd/taxonomy-server

run-search:
	go run ./cmd/search-server

run-audit:
	go run ./cmd/audit-server

run-posts:
	go run ./cmd/post-server

run-worker:
	go run ./cmd/outbox-worker

test:
	go test ./...

coverage:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@total=$$(go tool cover -func=coverage.out | awk '/^total:/ {gsub("%", "", $$NF); print $$NF}'); \
	awk -v total="$$total" 'BEGIN { if (total <= 25) { printf "coverage %.1f%% is below the 25%% project threshold\n", total; exit 1 } }'

fmt:
	gofmt -w ./cmd ./internal

vet:
	go vet ./...

build:
	go build -o bin/blog-server ./cmd/server

build-content:
	go build -o bin/content-server ./cmd/content-server

build-identity:
	go build -o bin/identity-server ./cmd/identity-server

build-comments:
	go build -o bin/comment-server ./cmd/comment-server

build-profile:
	go build -o bin/profile-server ./cmd/profile-server

build-analytics:
	go build -o bin/analytics-server ./cmd/analytics-server

build-taxonomy:
	go build -o bin/taxonomy-server ./cmd/taxonomy-server

build-search:
	go build -o bin/search-server ./cmd/search-server

build-audit:
	go build -o bin/audit-server ./cmd/audit-server

build-posts:
	go build -o bin/post-server ./cmd/post-server

build-worker:
	go build -o bin/outbox-worker ./cmd/outbox-worker
