# syntax=docker/dockerfile:1

ARG GO_VERSION=1.27
ARG NODE_VERSION=24
ARG DEBIAN_VERSION=trixie
ARG S6_OVERLAY_VERSION=3.2.3.2

# --- Web interface ----------------------------------------------------------
FROM --platform=$BUILDPLATFORM node:${NODE_VERSION}-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY web/ ./
RUN npm run build

# --- Server binary (cross-compiled, no emulation needed) --------------------
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS server
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY web/embed.go web/
COPY --from=web /src/web/dist web/dist
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
      -ldflags "-s -w \
        -X github.com/tchevalleraud/EasyNetServices/internal/version.Version=${VERSION} \
        -X github.com/tchevalleraud/EasyNetServices/internal/version.Commit=${COMMIT} \
        -X github.com/tchevalleraud/EasyNetServices/internal/version.Date=${DATE}" \
      -o /out/easynetservices ./cmd/easynetservices

# --- s6-overlay process supervisor ------------------------------------------
FROM --platform=$BUILDPLATFORM alpine:3 AS s6
ARG TARGETARCH
ARG S6_OVERLAY_VERSION
WORKDIR /tmp/s6
RUN set -eu; \
    case "$TARGETARCH" in \
      amd64) arch=x86_64 ;; \
      arm64) arch=aarch64 ;; \
      *) echo "unsupported architecture: $TARGETARCH" >&2; exit 1 ;; \
    esac; \
    base="https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}"; \
    mkdir -p /rootfs; \
    for pkg in noarch "$arch"; do \
      wget -q "$base/s6-overlay-$pkg.tar.xz" "$base/s6-overlay-$pkg.tar.xz.sha256"; \
      sha256sum -c "s6-overlay-$pkg.tar.xz.sha256"; \
      tar -C /rootfs -Jxpf "s6-overlay-$pkg.tar.xz"; \
    done

# --- Runtime image ----------------------------------------------------------
FROM debian:${DEBIAN_VERSION}-slim

# ca-certificates is needed to authenticate NTS time sources.
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates chrony \
    && rm -rf /var/lib/apt/lists/*

COPY --from=s6 /rootfs/ /
COPY docker/rootfs/ /
COPY --from=server /out/easynetservices /usr/local/bin/easynetservices

ENV S6_BEHAVIOUR_IF_STAGE2_FAILS=2 \
    S6_CMD_WAIT_FOR_SERVICES_MAXTIME=0 \
    S6_VERBOSITY=1 \
    ENS_DATA_DIR=/data

VOLUME /data
EXPOSE 8443/tcp 123/udp

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD ["/usr/local/bin/easynetservices", "healthcheck"]

ENTRYPOINT ["/init"]
