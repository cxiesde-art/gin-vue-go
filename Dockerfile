FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/blog-server ./cmd/server

FROM alpine:3.20
RUN adduser -D -H blog
USER blog
WORKDIR /app
COPY --from=build /out/blog-server .
EXPOSE 8080
ENTRYPOINT ["/app/blog-server"]
