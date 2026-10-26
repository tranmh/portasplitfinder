# Build the API binary.
FROM golang:1.22-alpine AS build
WORKDIR /src

# Cache module downloads.
COPY go.mod go.sum ./
RUN go mod download

# Copy the source and build a static binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/api ./cmd/api

# Final image: alpine so the binary can shell out for its own healthcheck
# (/app/api --healthcheck) and so we have a real /bin/sh for debugging.
FROM alpine:3.20
RUN apk add --no-cache ca-certificates \
    && addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=build /out/api /app/api
EXPOSE 8080
# Run as an unprivileged user, not root.
USER app
# DB migrations run on boot inside the binary.
ENTRYPOINT ["/app/api"]
