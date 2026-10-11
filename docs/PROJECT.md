# EasyNetServices — Project Definition

> Status: **draft** — under review, not yet frozen.

## 1. Vision

EasyNetServices is an all-in-one Docker image that provides the core network services
of a network — **DNS**, **DHCP** and **NTP** — and makes them easy to operate through a
web interface and a REST API.

It targets both:

- **Homelabs**: one container, sensible defaults, up and running in minutes.
- **Small and medium businesses**: several nodes for redundancy, managed from a single
  central interface, automation through the API.

EasyNetServices does not reimplement network protocols. It **orchestrates proven,
widely used open-source daemons** and exposes them through a unified management layer.

## 2. Functional scope

### 2.1 DNS

| Feature | Description |
|---|---|
| Authoritative zones | Host forward and reverse zones (primary). Full record management (A, AAAA, CNAME, MX, TXT, SRV, PTR, NS, CAA…). |
| Forward zones | Per-zone conditional forwarding to upstream servers (e.g. Active Directory domain, partner network). |
| Recursive resolution | Default recursion for everything not hosted or forwarded, with configurable upstreams (direct root resolution or forwarders, optional DoT). |
| Dynamic DNS | Automatic record registration from DHCP leases (v4 and v6), RFC 2136 updates secured by TSIG, and an HTTP update endpoint compatible with the `dyndns2` protocol used by routers and clients. |
| DNS firewall | Response Policy Zones (RPZ): subscription to external blocklists, custom block/allow rules, configurable actions (NXDOMAIN, NODATA, redirect). |
| Access control | Restrict which client networks can use recursion. |

### 2.2 DHCP

| Feature | Description |
|---|---|
| DHCPv4 and DHCPv6 | Both protocols supported. |
| Multiple subnets | Any number of subnets/pools, including relayed subnets (DHCP relay / VLANs). |
| Options | Standard options per server, subnet, pool or host (router, DNS, NTP, domain, PXE…). |
| Reservations | Static leases by MAC address, client ID or DUID. |
| Lease monitoring | View, search and release active leases. |
| High availability | Pair of nodes in hot-standby or load-balancing mode. |

### 2.3 NTP

| Feature | Description |
|---|---|
| NTP server | Serve time to the local network. |
| Upstream sources | Configurable upstream servers/pools. |
| Access control | Restrict which networks can query the server. |
| Status | Synchronization state, offset, sources. |

### 2.4 Management

| Feature | Description |
|---|---|
| Web UI | Configure and monitor every service from a browser. |
| REST API | Everything the UI can do is available through a documented API (OpenAPI). The UI itself only uses the public API. |
| API tokens | Personal access tokens for automation scripts and tools. |
| Authentication | Local accounts first. Default account `admin` / `password` created on first start, **password change enforced at first login**. Pluggable design so that LDAP and OIDC can be added later. |
| Configuration validation | Every change is validated before being applied; a failed apply never leaves a service broken. |
| Backup / restore | Export and import the full configuration. |

### 2.5 Multi-node (redundancy)

The same image can run in three roles:

| Role | Runs | Use case |
|---|---|---|
| `standalone` (default) | Controller + services | Homelab, single site. |
| `controller` | Web UI, API, configuration database | Central management of several nodes. |
| `node` | Services + local agent | Redundant service instances. |

- Nodes enroll with the controller using a one-time token, then communicate over
  authenticated TLS.
- Nodes **pull** their desired configuration from the controller (works behind NAT,
  no inbound port needed on nodes).
- Nodes keep serving with their last known configuration if the controller is unreachable.
- Redundancy per service: DNS and NTP by running the same configuration on several
  nodes; DHCP through the native high-availability mode of the DHCP server.

### 2.6 Out of scope (for now)

- Router advertisements (IPv6 RA) — normally handled by the router. May be added later.
- DNSSEC signing of hosted zones — candidate for a later version.
- Per-client DNS filtering policies — candidate for a later version.

## 3. Technical choices

### 3.1 Service engines

| Service | Engine | Why |
|---|---|---|
| Authoritative DNS | **PowerDNS Authoritative Server** | Native REST API, SQL backend, RFC 2136 dynamic updates with TSIG. |
| Recursive DNS | **PowerDNS Recursor** | Native RPZ support (DNS firewall), per-zone forwarding, REST API. Listens on port 53 and forwards hosted zones to the authoritative server internally. |
| DHCPv4 / DHCPv6 | **ISC Kea** | Modern successor of ISC DHCP, JSON configuration, management API, built-in HA, DHCP-DDNS daemon. |
| NTP | **chrony** | Lightweight, accurate, default NTP implementation on most Linux distributions. |

### 3.2 Management layer

| Component | Choice | Why |
|---|---|---|
| Backend / API / agent | **Go** | Single static binary, low footprint, strong concurrency, easy cross-compilation. The same binary serves as controller and node agent. |
| Frontend | **React + TypeScript + Vite** | Large ecosystem, typed, fast build. Compiled and embedded into the Go binary. |
| Database | **SQLite** | Zero configuration, a single file to back up. Abstracted so PostgreSQL can be added for larger deployments. |
| API specification | **OpenAPI 3** | Generated documentation, client generation for automation. |
| Process supervision | **s6-overlay** | Proper multi-process supervision and signal handling inside one container. |
| Base image | **Debian (stable, slim)** | Official upstream packages for Kea and PowerDNS. |

### 3.3 Container

- Target architectures: `linux/amd64` and `linux/arm64` (Raspberry Pi 4/5 and ARM servers).
- DHCP requires direct access to the network: recommended mode is `network_mode: host`.
- Persistent data in a single volume: `/data` (database, leases, certificates).
- Configuration of the container itself through environment variables (role,
  controller URL, enrollment token, listen addresses…).

## 4. Versioning and delivery

- [Semantic Versioning](https://semver.org/): `MAJOR.MINOR.PATCH`.
- Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/),
  which drives the automatic changelog and version bump.
- Docker images published on Docker Hub:

| Event | Image tags |
|---|---|
| Push on `dev` | `dev`, `dev-<short-sha>` |
| Release `vX.Y.Z` | `X.Y.Z`, `X.Y`, `X`, `latest` |

See [CONTRIBUTING.md](../CONTRIBUTING.md) for the development workflow.

## 5. Open questions

- License to confirm.
- Docker Hub repository name.
