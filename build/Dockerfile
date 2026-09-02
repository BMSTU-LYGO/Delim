FROM golang:1.26-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG SERVICE
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/service ./cmd/${SERVICE}

FROM alpine:3.23

RUN addgroup -S app && adduser -S -G app app
WORKDIR /app

COPY --from=builder /out/service /app/service

USER app
ENTRYPOINT ["/app/service"]
