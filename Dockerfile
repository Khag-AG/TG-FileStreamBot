FROM golang:1.23-alpine AS builder
RUN apk update && apk upgrade --available && sync
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o /app/fsb -ldflags="-w -s" ./cmd/fsb

FROM scratch
COPY --from=builder /app/fsb /app/fsb
EXPOSE ${PORT}
ENTRYPOINT ["/app/fsb", "run"]
