# Build the poller binary.
FROM golang:1.22-alpine AS build
WORKDIR /src

# Cache module downloads.
COPY go.mod go.sum ./
RUN go mod download

# Copy the source and build a static binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/poller ./cmd/poller

# Final image: alpine so /app/poller --healthcheck can run.
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/poller /app/poller
EXPOSE 8090
ENTRYPOINT ["/app/poller"]
