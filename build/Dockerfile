FROM golang:1.26-alpine AS build-base

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Compile the two production services together. They share most dependencies, so
# a single Go invocation compiles that graph once instead of once per Compose
# service. BuildKit also executes this stage only once when both targets build.
FROM build-base AS services-builder
RUN mkdir /out && \
    CGO_ENABLED=0 GOOS=linux go build \
      -trimpath -ldflags="-s -w" -o /out ./cmd/core ./cmd/gateway

# The development-only MinIO initializer does not belong in production builds.
FROM build-base AS minioinit-builder
RUN CGO_ENABLED=0 GOOS=linux go build \
      -trimpath -ldflags="-s -w" -o /out/service ./cmd/minioinit

FROM alpine:3.23 AS runtime

RUN addgroup -S app && adduser -S -G app app
WORKDIR /app

USER app
ENTRYPOINT ["/app/service"]

FROM runtime AS core
COPY --from=services-builder /out/core /app/service

FROM runtime AS gateway
COPY --from=services-builder /out/gateway /app/service

FROM runtime AS minioinit
COPY --from=minioinit-builder /out/service /app/service
