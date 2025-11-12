# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o kiln ./cmd/kiln

# Runtime stage
FROM alpine:latest

# Install ca-certificates and chromium for Rod
RUN apk add --no-cache ca-certificates chromium

# Create non-root user
RUN addgroup -S kiln && adduser -S kiln -G kiln

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/kiln .

# Change ownership
RUN chown -R kiln:kiln /app

# Switch to non-root user
USER kiln

# Expose port
EXPOSE 8080

# Run the application
CMD ["./kiln"]
