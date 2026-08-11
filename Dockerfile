# Stage 1: Build statically linked binary
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install certificates
RUN apk add --no-cache ca-certificates git

# Copy Go modules manifests and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy application source code
COPY . .

# Build lightweight production binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o trueline-server ./cmd/server/main.go

# Stage 2: Production runtime container
FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

# Copy built binary from builder stage
COPY --from=builder /app/trueline-server .

# Default environment settings
ENV PORT=8080
ENV ENV=production

EXPOSE 8080

CMD ["./trueline-server"]
