# Build stage
FROM golang:1.24.3-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum first to take advantage of Docker cache
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source
COPY . .

# Build the binary
RUN go build -o webhook-server main.go

# Runtime stage (smaller)
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/webhook-server .

EXPOSE 443

CMD ["./webhook-server"]

