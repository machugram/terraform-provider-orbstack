# ADR 0005: Keep go test offline

Date: 2026-09-30
Status: Accepted

## Context

This provider only does useful work on a Mac with OrbStack running. Acceptance tests that create a VM or pull nginx would prove the integration, and they would also fail in CI, on Linux, and on a laptop whose Docker socket is down. They can also delete or restart a machine the developer already uses.

The bugs that hurt this codebase are local and pure: a missing `--disk` flag, a `0` limit stored as `0` instead of null, a host port parsed as the container port, a port of `70000` accepted.

## Decision

`go test ./...` does not execute `orbctl`, dial the Docker socket, or pull an image.

Coverage required for a change to the clients:

- create argv, including omitted flags
- decoding `orbctl info -f json`
- `0` cpu, memory, and disk bytes become nil; non-zero disk bytes convert back to GiB
- `BuildSpec` maps ports, env, binds, NanoCPUs, memory, and restart policy onto Docker types
- a port outside 1–65535 returns an error that does not contain environment values

Live apply is a manual check with a dev override, documented in [development.md](../development.md). Any future acceptance test must be behind an explicit env var and must use a disposable name.

## Consequences

- CI can run `go test` anywhere Go runs.
- A green test does not prove `orbctl create` still accepts `--memory 2048M`. The argv test pins the flag we send. A CLI change still needs a human or a gated acceptance test.
- Developers are not one `go test` away from deleting the `ubuntu` machine they already have.

## Options considered

- Acceptance tests in the default `go test`. Highest fidelity, and the default command becomes unsafe and environment-specific.
- Recorded HTTP fixtures for the Docker client. Useful later for `CreateContainer` error paths. They do not replace the pure spec test, and the machine path is a CLI rather than HTTP.
