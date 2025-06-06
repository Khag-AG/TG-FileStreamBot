FROM golang:1.23-alpine AS builder
RUN apk update && apk upgrade --available && sync
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o /app/fsb -ldflags="-w -s" ./cmd/fsb

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/fsb /app/fsb
COPY --from=builder /app/internal/admin/static /app/internal/admin/static
WORKDIR /app
ENTRYPOINT ["/app/fsb", "run"]