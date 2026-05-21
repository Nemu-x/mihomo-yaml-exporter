FROM golang:1.23-alpine AS build
RUN apk add --no-cache ca-certificates git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /mihomo-yaml-exporter ./cmd/exporter

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /mihomo-yaml-exporter /usr/local/bin/mihomo-yaml-exporter
EXPOSE 9123
USER nobody
ENTRYPOINT ["/usr/local/bin/mihomo-yaml-exporter"]