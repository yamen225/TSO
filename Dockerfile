# ---- Build Stage ----
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /activation-service ./cmd/server

# ---- Runtime Stage ----
FROM alpine:3.19

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /activation-service .

EXPOSE 8080

ENTRYPOINT ["/app/activation-service"]
