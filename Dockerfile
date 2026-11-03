FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG BINARY=blog-server
ARG COMMAND=./cmd/server
RUN CGO_ENABLED=0 go build -o /out/${BINARY} ${COMMAND}

FROM alpine:3.20
RUN adduser -D -H blog
USER blog
WORKDIR /app
ARG BINARY=blog-server
COPY --from=build /out/${BINARY} /app/${BINARY}
EXPOSE 8080
ENTRYPOINT ["/app/blog-server"]
