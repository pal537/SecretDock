# ---- build ----
FROM golang:1.24-alpine AS builder
WORKDIR /src
# Pure-Go SQLite (modernc.org/sqlite): no CGO needed, cross-compiles cleanly.
ENV CGO_ENABLED=0
# Allow automatic toolchain upgrades (go.mod may require a newer Go).
ENV GOTOOLCHAIN=auto
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG APP_VERSION=dev
RUN go build -trimpath -ldflags="-s -w -X main.appVersion=$APP_VERSION" -o /secretdock .

# ---- runtime ----
# Note: runs as root so it can write to the /data volume on any host.
FROM gcr.io/distroless/static-debian12
COPY --from=builder /secretdock /secretdock
# IP country database for the login-security country allowlist
# (a fresher copy can be downloaded from settings into /data/geoip/).
COPY --from=builder /src/geoip/dbip-country-lite.mmdb /geoip/dbip.mmdb
# Frontend is embedded in the binary via go:embed; no extra files needed.
EXPOSE 8080
VOLUME ["/data"]
ENV SECRETDOCK_DATA_DIR=/data
ENV SECRETDOCK_ADDR=0.0.0.0:8080
ENTRYPOINT ["/secretdock"]
