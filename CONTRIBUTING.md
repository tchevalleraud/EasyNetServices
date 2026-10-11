# Contributing

## Languages

All code, comments, commit messages and documentation are written in **English**.

## Repository layout

| Path | Content |
|---|---|
| `cmd/easynetservices` | Entry point of the server binary |
| `internal/` | Backend packages (API, authentication, storage…) |
| `web/` | Web interface (React, TypeScript, Vite), embedded into the binary |
| `api/openapi.yaml` | REST API specification, kept in sync with the handlers |
| `docker/rootfs` | Files copied into the image (s6-overlay service definitions) |
| `docs/` | Project documentation |

## Local development

The quickest way to run the project locally only requires Docker: it builds the
image from the working tree and starts it with the host network on
`https://localhost:8443`.

```sh
make up        # build and start (compose.dev.yaml)
make logs      # follow the logs
make down      # stop
```

Every change is checked on this local stack before being pushed.

For a native setup: Go (version in `go.mod`), Node.js 24, Docker with Buildx.

```sh
make test      # backend tests
make lint      # formatting and linters
make run       # build and run the server on https://localhost:8443 (data in ./data)
make docker    # build the image for the local architecture
```

For frontend work, run the server with `make run`, then start the Vite dev server
with hot reload in another terminal; it proxies `/api` to the local server:

```sh
cd web && npm run dev
```

## Branches

| Branch | Purpose |
|---|---|
| `main` | Stable, released code only. Every release is a tag on `main`. |
| `dev` | Integration branch. All development lands here. |
| `feat/*`, `fix/*`, `chore/*`… | Short-lived branches created from `dev`, merged back into `dev` through a pull request. |

Direct pushes to `main` are not allowed; direct pushes to `dev` are tolerated
during the early phase of the project.

## Commit messages

Commits follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<optional scope>): <description>
```

Common types: `feat`, `fix`, `docs`, `refactor`, `test`, `build`, `ci`, `chore`.
Common scopes: `dns`, `dhcp`, `ntp`, `api`, `ui`, `auth`, `agent`, `docker`.

Examples:

```
feat(dhcp): add DHCPv6 reservations
fix(dns): reject CNAME at zone apex
docs: describe multi-node enrollment
```

A breaking change is marked with `!` (e.g. `feat(api)!: rename /zones endpoint`)
and triggers a major version bump.

## Versioning

The project follows [Semantic Versioning](https://semver.org/). Versions and the
changelog are derived from commit messages by
[release-please](https://github.com/googleapis/release-please); versions are never
edited by hand. Before 1.0.0, a `feat` bumps the minor version and a `fix` the
patch version.

## Release process

1. Open a pull request from `dev` to `main` and merge it.
2. release-please opens (or updates) a release pull request on `main` with the
   new version and the changelog.
3. Merging the release pull request creates the `vX.Y.Z` tag and the GitHub
   release, and publishes the Docker image.
4. Merge `main` back into `dev` so that it contains the changelog and version files.

## Continuous integration

GitHub Actions run on every pull request and on every push to `dev`:

- lint and tests (backend and frontend),
- multi-architecture Docker build (`linux/amd64`, `linux/arm64`),
- image publication to Docker Hub (`tchevalleraud91/easynetservices`) on `dev`
  and on releases.

Publishing requires the `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` repository
secrets.
