FROM golang:1.24.4-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app
COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o smartheart ./cmd

FROM alpine:latest

RUN apk --no-cache add ca-certificates curl jq

RUN addgroup -g 1001 -S smartheart && \
    adduser -u 1001 -S smartheart -G smartheart

WORKDIR /app

COPY --from=builder /app/smartheart .

COPY --from=builder /app/migrations ./migrations

RUN chown -R smartheart:smartheart /app

USER smartheart

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/v1/auth/login || exit 1

EXPOSE 8080

CMD ["./smartheart"]
