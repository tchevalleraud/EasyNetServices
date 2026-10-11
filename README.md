# EasyNetServices

> Core network services (DNS, DHCP, NTP) in a single Docker image, managed through a web interface.

**Status:** early design phase — not ready for use yet.

## Overview

EasyNetServices aims to provide a ready-to-run Docker image bundling the essential
network services of a small or medium network, with a graphical web interface to
configure and monitor them without editing configuration files by hand.

## Planned features

- **DNS** — authoritative zones and recursive resolution
- **DHCP** — scopes, reservations and lease monitoring
- **NTP** — time server for the local network
- **Web UI** — configure and monitor every service from a browser
- **Single container** — deploy with one `docker run` or `docker compose up`

## Roadmap

The project scope, architecture and technical choices are being defined in
[docs/PROJECT.md](docs/PROJECT.md). See [CONTRIBUTING.md](CONTRIBUTING.md) for the
development workflow.

## License

To be defined.
