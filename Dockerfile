# Build stage
FROM golang:1.24-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS and bash for scripts
RUN apk --no-cache add ca-certificates bash curl

# Set working directory
WORKDIR /app/

# Copy the binary from builder stage
COPY --from=builder /app/main .
COPY config ./

# Create directory for Kubernetes config
RUN mkdir -p /app/.kube

# Expose port
EXPOSE 8080

# Set environment variables
ENV GIN_MODE=release

# Run the application with in-cluster mode by default
ENTRYPOINT ["bash"]
