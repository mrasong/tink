FROM golang:1.27-alpine AS builder

WORKDIR /app
COPY server/go.mod server/go.sum ./
RUN go mod download

COPY server/ .
COPY VERSION /app/VERSION
ARG VERSION
ARG BUILD_ID
RUN if [ -z "$BUILD_ID" ]; then BUILD_ID=unknown; fi && \
  if [ -z "$VERSION" ]; then VERSION=$(tr -d '[:space:]' < /app/VERSION); fi && \
  CGO_ENABLED=0 GOOS=linux go build \
  -ldflags="-w -s -X main.Version=$VERSION -X main.Build=$BUILD_ID -X github.com/mrasong/tink/server/internal/api.Version=$VERSION -X github.com/mrasong/tink/server/internal/api.Build=$BUILD_ID" \
  -o /tink-server ./cmd/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app
COPY --from=builder /tink-server /app/tink-server

VOLUME /data
EXPOSE 5021

ENV TINK_DATA_DIR=/data
ENV TINK_PORT=5021

ENTRYPOINT ["/app/tink-server", "serve"]
