# Contributing

## Languages

All code, comments, commit messages and documentation are written in **English**.

## Branches

| Branch | Purpose |
|---|---|
| `main` | Stable, released code only. Every release is a tag on `main`. |
| `dev` | Integration branch. All development lands here. |
| `feat/*`, `fix/*`, `chore/*`… | Short-lived branches created from `dev`, merged back into `dev` through a pull request. |

Releases are made by merging `dev` into `main`.

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
changelog are derived from commit messages; versions are never edited by hand.

## Continuous integration

GitHub Actions run on every pull request and push:

- lint and tests (backend and frontend),
- multi-architecture Docker build (`linux/amd64`, `linux/arm64`),
- image publication to Docker Hub on `dev` and on releases.
