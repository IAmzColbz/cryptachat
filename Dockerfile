# Stage 1: Builder
# Use an official Go image. Using 1.22-alpine as an example.
FROM golang:1.22-alpine AS builder
WORKDIR /app

# Copy module files and download dependencies first
# This leverages Docker's build cache
COPY src/go.mod src/go.sum ./
RUN go mod download

# Copy the rest of the Go source code
# This copies main.go, config/, http/, store/ into /app
COPY src/ ./ 

# Copy the schema file that the binary needs to read on startup
COPY server/schema.sql ./server/schema.sql

# Build the static, production-ready binary
# CGO_ENABLED=0 creates a static binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /cryptachat-server ./main.go

# Stage 2: Final Image
# Use a minimal base image for a small footprint
FROM alpine:latest
WORKDIR /app

# Copy the built binary from the builder stage
COPY --from=builder /cryptachat-server .

# Copy the schema file again, placing it relative to the binary
COPY --from=builder /app/server/schema.sql ./server/schema.sql

# Expose the port the Go server listens on
EXPOSE 5000

# Run the server
CMD ["./cryptachat-server"]