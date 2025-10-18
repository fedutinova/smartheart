FROM golang:1.24.4-alpine AS builder

# Install OpenCV dependencies
RUN apk add --no-cache git ca-certificates tzdata \
    pkgconfig \
    opencv-dev \
    build-base

WORKDIR /app
COPY go.mod go.sum ./

RUN go mod download

COPY . .

# Build with CGO enabled for OpenCV
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o smartheart ./cmd

FROM alpine:latest

# Install OpenCV runtime dependencies
RUN apk --no-cache add ca-certificates curl jq \
    opencv \
    libgomp

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
