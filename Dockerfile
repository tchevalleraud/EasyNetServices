# syntax=docker/dockerfile:1

ARG GO_VERSION=1.27
ARG NODE_VERSION=24
ARG DEBIAN_VERSION=trixie
ARG S6_OVERLAY_VERSION=3.2.3.2
# PowerDNS release series, from the official repositories.
ARG PDNS_AUTH_SERIES=51
ARG PDNS_REC_SERIES=54
# Kea release series, from the official ISC repositories.
ARG KEA_SERIES=3-2

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

# --- PowerDNS repository signing key ----------------------------------------
FROM --platform=$BUILDPLATFORM alpine:3 AS pdns-key
ARG PDNS_KEY_FINGERPRINT=9FAAA5577E8FCF62093D036C1B0C6205FD380FBB
RUN apk add --no-cache gnupg \
    && wget -q -O /powerdns.asc https://repo.powerdns.com/FD380FBB-pub.asc \
    && gpg --show-keys --with-colons /powerdns.asc | grep -q "^fpr:*${PDNS_KEY_FINGERPRINT}:"

# --- Kea repository signing key ---------------------------------------------
FROM --platform=$BUILDPLATFORM alpine:3 AS kea-key
ARG KEA_SERIES
ARG KEA_KEY_FINGERPRINT=360D0B03DFE219DB0F9FEA4C720DD962343B440F
RUN apk add --no-cache gnupg \
    && wget -q -O /isc-kea.asc "https://dl.cloudsmith.io/public/isc/kea-${KEA_SERIES}/gpg.key" \
    && gpg --show-keys --with-colons /isc-kea.asc | grep -q "^fpr:*${KEA_KEY_FINGERPRINT}:"

# --- Runtime image ----------------------------------------------------------
FROM debian:${DEBIAN_VERSION}-slim
ARG DEBIAN_VERSION
ARG PDNS_AUTH_SERIES
ARG PDNS_REC_SERIES
ARG KEA_SERIES

COPY --from=pdns-key /powerdns.asc /etc/apt/keyrings/powerdns.asc
COPY --from=kea-key /isc-kea.asc /etc/apt/keyrings/isc-kea.asc

# ca-certificates is needed to authenticate NTS time sources, DNS over TLS and
# the HTTPS repository of Kea.
RUN set -eu; \
    apt-get update; \
    apt-get install -y --no-install-recommends ca-certificates; \
    repo="deb [signed-by=/etc/apt/keyrings/powerdns.asc] http://repo.powerdns.com/debian"; \
    echo "$repo ${DEBIAN_VERSION}-auth-${PDNS_AUTH_SERIES} main" > /etc/apt/sources.list.d/powerdns-auth.list; \
    echo "$repo ${DEBIAN_VERSION}-rec-${PDNS_REC_SERIES} main" > /etc/apt/sources.list.d/powerdns-rec.list; \
    printf 'Package: pdns-*\nPin: origin repo.powerdns.com\nPin-Priority: 600\n' > /etc/apt/preferences.d/powerdns; \
    echo "deb [signed-by=/etc/apt/keyrings/isc-kea.asc] https://dl.cloudsmith.io/public/isc/kea-${KEA_SERIES}/deb/debian ${DEBIAN_VERSION} main" \
      > /etc/apt/sources.list.d/isc-kea.list; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
      chrony sqlite3 \
      pdns-server pdns-backend-sqlite3 pdns-recursor \
      isc-kea-dhcp4 isc-kea-dhcp6 isc-kea-hooks libcap2-bin; \
    # Kea runs as an unprivileged user: grant its binaries the capabilities to
    # open raw sockets and bind the DHCP ports. libcap2-bin provides setcap and
    # is required by chrony anyway, through iproute2.
    setcap cap_net_raw,cap_net_bind_service+ep /usr/sbin/kea-dhcp4; \
    setcap cap_net_bind_service+ep /usr/sbin/kea-dhcp6; \
    rm -rf /var/lib/apt/lists/*

COPY --from=s6 /rootfs/ /
COPY docker/rootfs/ /
COPY --from=server /out/easynetservices /usr/local/bin/easynetservices

ENV S6_BEHAVIOUR_IF_STAGE2_FAILS=2 \
    S6_CMD_WAIT_FOR_SERVICES_MAXTIME=0 \
    S6_VERBOSITY=1 \
    ENS_DATA_DIR=/data

VOLUME /data
EXPOSE 8443/tcp 53/udp 53/tcp 67/udp 123/udp 547/udp

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD ["/usr/local/bin/easynetservices", "healthcheck"]

ENTRYPOINT ["/init"]
