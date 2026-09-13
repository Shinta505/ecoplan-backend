# ==========================================
# Tahap 1: Builder
# ==========================================
FROM golang:1.27.1-alpine3.24 AS builder

WORKDIR /app

# Dependency Go
COPY go.mod go.sum ./

RUN go mod download

# Source code
COPY . .

# Build binary untuk Linux AMD64
RUN CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    go build \
    -ldflags="-w -s" \
    -o ecoplan-binary \
    ./cmd/main.go


# ==========================================
# Tahap 2: Production Runtime
# ==========================================
FROM alpine:3.24

# Update package dan install kebutuhan runtime
RUN apk update && \
    apk upgrade --no-cache && \
    apk add --no-cache \
        ca-certificates \
        tzdata

# Zona waktu
ENV TZ=Asia/Jakarta

WORKDIR /app

# Binary aplikasi
COPY --from=builder /app/ecoplan-binary .

# Template HTML dokumentasi API
COPY --from=builder /app/views ./views

# Port Cloud Run
EXPOSE 8080

# Jalankan aplikasi
CMD ["./ecoplan-binary"]