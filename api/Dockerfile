# Frontend build stage
FROM node:22-alpine AS frontend
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN npm run build

# Build stage
FROM golang:1.24-alpine3.22 AS builder

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Copy built frontend assets
COPY --from=frontend /web/dist ./web/dist

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Final stage
FROM alpine:3.22

# Install ca-certificates for HTTPS and bash for scripts
RUN apk --no-cache add ca-certificates bash curl

# Set working directory
WORKDIR /app/

# Copy the binary from builder stage
COPY --from=builder /app/main .
COPY ./config/dm_config.default.toml /app/config/dm_config.toml
COPY ./config/manager_config.default.toml /app/config/manager_config.toml
COPY ./manager/migration /app/manager/migration

# Create directory for Kubernetes config
RUN mkdir -p /app/.kube

# Expose port
CMD [ "/app/main", "manager" ]