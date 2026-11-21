# --- Build Stage ---
FROM golang:1.27.4-bookworm AS builder

WORKDIR /app

ENV NOTES_SERVICE_PORT=8080

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and compile the binary
COPY . .
RUN go build -o /myapp ./cmd/notes-api

# --- Runtime Stage ---
FROM debian:bookworm-slim

WORKDIR /app

# Copy the compiled binary from the builder stage
COPY --from=builder /myapp /app/myapp

# Expose the default application port
EXPOSE 8080

CMD ["/app/myapp"]
