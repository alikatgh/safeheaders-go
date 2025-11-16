# Multi-stage Dockerfile for SafeHeaders-Go

# Stage 1: Build
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go.work and module files first (for better caching)
COPY go.work go.work
COPY go.mod go.mod

# Copy all module directories
COPY jsmn-go/ jsmn-go/
COPY stb-image-go/ stb-image-go/
COPY stb-truetype-go/ stb-truetype-go/
COPY tinyxml2-go/ tinyxml2-go/
COPY cjson-go/ cjson-go/
COPY miniz-go/ miniz-go/
COPY cgltf-go/ cgltf-go/
COPY dr-wav-go/ dr-wav-go/
COPY examples/ examples/

# Download dependencies
RUN cd jsmn-go && go mod download

# Build examples
RUN mkdir -p /app/bin && \
    cd examples/json-parser && go build -ldflags="-s -w" -o /app/bin/json-parser main.go && \
    cd ../production-usage && go build -ldflags="-s -w" -o /app/bin/production-server main.go

# Stage 2: Runtime
FROM alpine:latest

# Install CA certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Set working directory
WORKDIR /app

# Copy binaries from builder
COPY --from=builder /app/bin/* /app/

# Copy test data
COPY --from=builder /app/testdata /app/testdata

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Default command (can be overridden)
CMD ["/app/production-server"]

# Expose port for production server
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1
