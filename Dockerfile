# build stage
FROM golang:1.24 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/docsvc ./server

# runtime stage
FROM debian:stable-slim
RUN apt-get update && apt-get install -y --no-install-recommends libreoffice && rm -rf /var/lib/apt/lists/*
WORKDIR /srv
COPY --from=builder /out/docsvc /usr/local/bin/docsvc
EXPOSE 5051 9090
ENTRYPOINT ["/usr/local/bin/docsvc"]
