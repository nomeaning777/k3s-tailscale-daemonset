# Build stage
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o app cmd/main.go

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache \
    tailscale \
    iproute2 \
    ca-certificates \
    && rm -rf /var/cache/apk/*

COPY --from=builder /build/app /app/main

RUN chmod +x /app/main

ENTRYPOINT ["/app/main"]