FROM golang:1.21-alpine AS builder

WORKDIR /build

# Install git (required for go modules)
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o pilot-swe ./cmd

# Final stage
FROM alpine:latest

# Install git and gh CLI
RUN apk add --no-cache git github-cli

# Copy binary from builder
COPY --from=builder /build/pilot-swe /usr/local/bin/pilot-swe

# Expose port
EXPOSE 3000

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:3000/health || exit 1

# Run the application
ENTRYPOINT ["pilot-swe"]
