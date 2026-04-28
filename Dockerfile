FROM golang:1.26-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -o bin/bucket-brigade ./cmd/bucket-brigade

FROM alpine:3.21

RUN apk add --no-cache ca-certificates
WORKDIR /app

COPY --from=builder /app/bin/bucket-brigade /app/bucket-brigade
COPY properties.yaml .
COPY db/migrations ./db/migrations
ENV DATABASE_SQLITE_PATH=/data/bucket_brigade.db \
    STORAGE_BASE_PATH=/data

RUN mkdir -p /data \
    && addgroup -S brigade \
    && adduser -S brigade -G brigade \
    && chown -R brigade:brigade /app /data

USER brigade
EXPOSE 8080
ENTRYPOINT ["/app/bucket-brigade"]
