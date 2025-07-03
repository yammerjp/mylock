# Build stage
FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o mylock ./cmd/mylock

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

COPY --from=builder /app/mylock /usr/local/bin/mylock

# Create non-root user
RUN addgroup -g 1000 -S mylock && \
    adduser -u 1000 -S mylock -G mylock

USER mylock

ENTRYPOINT ["mylock"]