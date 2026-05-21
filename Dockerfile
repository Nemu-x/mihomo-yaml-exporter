FROM golang:1.23-alpine AS build
RUN apk add --no-cache ca-certificates git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /mihomo-yaml-exporter ./cmd/exporter

FROM alpine:3.20
ARG MIHOMO_VERSION=v1.19.25
RUN apk add --no-cache ca-certificates wget \
    && wget -qO- "https://github.com/MetaCubeX/mihomo/releases/download/${MIHOMO_VERSION}/mihomo-linux-amd64-compatible-${MIHOMO_VERSION}.gz" \
    | gunzip > /usr/local/bin/mihomo \
    && chmod +x /usr/local/bin/mihomo \
    && apk del wget
COPY --from=build /mihomo-yaml-exporter /usr/local/bin/mihomo-yaml-exporter
EXPOSE 9123
USER nobody
ENTRYPOINT ["/usr/local/bin/mihomo-yaml-exporter"]
