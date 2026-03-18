# Build stage
FROM golang:1.24.5-alpine AS builder

WORKDIR /app

# Install git and ca-certificates
RUN apk add --no-cache git ca-certificates \
    && go install github.com/swaggo/swag/cmd/swag@v1.16.6

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Generate Swagger docs
RUN swag init -g cmd/main.go

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/main .

# Copy configuration files
COPY --from=builder /app/configs ./configs

# Copy Swagger docs
COPY --from=builder /app/docs ./docs

# Expose port (if needed)
# EXPOSE 8080

# Run the application
CMD ["./main"] 