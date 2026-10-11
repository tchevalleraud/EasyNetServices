# EasyNetServices

> Core network services (DNS, DHCP, NTP) in a single Docker image, managed through a web interface.

**Status:** early development — not ready for production use yet.

## Overview

EasyNetServices aims to provide a ready-to-run Docker image bundling the essential
network services of a small or medium network, with a graphical web interface to
configure and monitor them without editing configuration files by hand.

## Planned features

- **DNS** — authoritative zones and recursive resolution, dynamic DNS, DNS firewall
- **DHCP** — DHCPv4 and DHCPv6 subnets, reservations and lease monitoring
- **NTP** — time server for the local network
- **Web UI and REST API** — configure and monitor every service from a browser or a script
- **Single container** — deploy with one `docker run` or `docker compose up`
- **Redundancy** — several nodes managed from a single central interface

## Quick start

```sh
docker run -d --name easynetservices \
  --network host \
  -v easynetservices-data:/data \
  --restart unless-stopped \
  tchevalleraud91/easynetservices:latest
```

Or with Docker Compose, using the provided [compose.yaml](compose.yaml):

```sh
docker compose up -d
```

Then open `https://<host>:8443` and sign in with the default account
`admin` / `password`. You will be asked to choose a new password at first login.

> The web interface uses a self-signed certificate by default, so your browser
> will show a warning on the first visit.

### Image tags

| Tag | Content |
|---|---|
| `latest` | Latest release |
| `X.Y.Z`, `X.Y` | A specific release |
| `dev` | Latest build of the development branch (unstable) |

Images are available for `linux/amd64` and `linux/arm64`.

## Configuration

The container is configured through environment variables:

| Variable | Default | Description |
|---|---|---|
| `ENS_ROLE` | `standalone` | Role of the container. Only `standalone` is available for now. |
| `ENS_DATA_DIR` | `/data` | Directory holding the database and certificates. |
| `ENS_LISTEN` | `:8443` | Listen address of the web interface and API. |
| `ENS_TLS_MODE` | `self-signed` | `self-signed`, `custom` (use `ENS_TLS_CERT`/`ENS_TLS_KEY`) or `off` (plain HTTP behind a reverse proxy). |
| `ENS_TLS_CERT` | | Path to the certificate file when `ENS_TLS_MODE=custom`. |
| `ENS_TLS_KEY` | | Path to the private key file when `ENS_TLS_MODE=custom`. |
| `ENS_SESSION_TTL` | `12h` | Lifetime of a web session, extended while it is in use. |
| `ENS_LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`. |
| `TZ` | `UTC` | Initial time zone (e.g. `Europe/Paris`), used until one is chosen in the web interface. |
| `ENS_NTP_CLOCK_CONTROL` | `false` | Let the NTP server discipline the host clock. Requires `--cap-add SYS_TIME`. |

### Ports

| Port | Service |
|---|---|
| `8443/tcp` | Web interface and API |
| `123/udp` | NTP |

### NTP and the host clock

The container shares the clock of the host. By default the NTP server only
serves time: it tracks its upstream sources but never adjusts the clock, which
should be synchronized by the host itself. To let EasyNetServices synchronize
the host clock instead, set `ENS_NTP_CLOCK_CONTROL=true` and add the `SYS_TIME`
capability (`--cap-add SYS_TIME`, or `cap_add: [SYS_TIME]` with Compose).

### Lost password

Set a temporary password on an account from the host:

```sh
docker exec easynetservices easynetservices reset-password admin
```

## API

The REST API is described in [api/openapi.yaml](api/openapi.yaml).

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) for the development workflow and
[docs/PROJECT.md](docs/PROJECT.md) for the project scope and architecture.

## License

Released under the [Apache License 2.0](LICENSE).
