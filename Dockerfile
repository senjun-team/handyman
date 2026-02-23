FROM golang:1.18-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o handyman ./cmd/handyman

# Runtime image
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary
COPY --from=builder /build/handyman /app/

# Copy courses directory (will be mounted as volume in docker-compose)
# This is just to ensure the directory exists
RUN mkdir -p /data/courses

EXPOSE 8080

# Run handyman with courses path
CMD ["./handyman", "/data/courses"]


