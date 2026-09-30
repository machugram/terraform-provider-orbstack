# ADR 0002: Use orbctl for machines and the Docker Engine API for containers

Date: 2026-09-30
Status: Accepted

## Context

OrbStack exposes two control planes on one Mac.

Linux machines are an `orbctl` feature. `orbctl info -f json` and `orbctl list -f json` return stable ids, image, state, and config. Limits live in `orbctl config` keys such as `machine.<name>.cpu`. There is no documented HTTP API for machines.

Containers are Docker. OrbStack tells you to use the `docker` CLI, and it publishes a Docker socket at `~/.orbstack/run/docker.sock`. The context name is `orbstack`.

## Decision

Machines go through `orbctl`. The client accepts JSON only. Text scraping of `orb info` is not a parser we are willing to keep compatible.

Containers go through `github.com/moby/moby/client` against that socket. The provider does not shell out to `docker run`. Inspect, create, start, stop, and remove return structured errors, including not-found, which is what Terraform refresh needs.

`docker_host` resolution order is: explicit configuration, `docker context inspect orbstack`, then the default socket path. Resolution does not dial. The first container RPC does, and a connection error names the host it tried.

## Consequences

- Machine behavior tracks the CLI. A new `orbctl` flag is a client change with a unit test on argv.
- Container behavior tracks the Engine API. Port bindings and restart policy are real Docker fields, not a second CLI dialect.
- The provider requires `orbctl` and a running OrbStack for apply. Unit tests require neither.
- We do not wrap the `docker` CLI. That would make inspect output another parser.

## Options considered

- Shell out to `docker` for containers. Fewer dependencies, and a worse Read implementation. The Engine API is the contract Docker already documents.
- Reimplement machines on an undocumented OrbStack socket. No stable schema, and `orbctl` already emits JSON.
