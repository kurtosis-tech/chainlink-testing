FROM golang:1.15-alpine AS builder
WORKDIR /build
# Copy and download dependencies using go mod
COPY go.mod .
COPY go.sum .
RUN go mod download

# Copy the code into the container
COPY . .

# Build the application
RUN go build -o price-feed-server.bin ./price_feed_server/main.go

# ============= Execution Stage ================
FROM alpine:3.12 AS execution

WORKDIR /run

# Copy the code into the container
COPY --from=builder /build/price-feed-server.bin .

CMD ./price-feed-server.bin
