# build stage
FROM golang:1.24 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/docsvc ./server
# download grpc_health_probe (binary resmi)
ADD https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.21/grpc_health_probe-linux-amd64 /bin/grpc_health_probe
RUN chmod +x /bin/grpc_health_probe

# runtime stage
FROM debian:stable-slim
RUN apt-get update && apt-get install -y --no-install-recommends libreoffice && rm -rf /var/lib/apt/lists/*
WORKDIR /srv
COPY --from=builder /out/docsvc /usr/local/bin/docsvc
COPY --from=builder /bin/grpc_health_probe /bin/grpc_health_probe
EXPOSE 5051 9090

HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD ["/bin/grpc_health_probe", "-addr=:5051"]
ENTRYPOINT ["/usr/local/bin/docsvc"]
