# ADR 0001: Keep upstream clients independent of Terraform

Date: 2026-09-30
Status: Accepted

## Context

A Terraform provider has two jobs that change for different reasons. The schema has to match practitioner expectations: defaults, `RequiresReplace`, import, and diagnostics. The upstream calls have to match OrbStack and the Docker Engine: argv, JSON, pull, inspect, and socket selection.

Putting both in `internal/provider` makes the resources the only place that knows how a machine is created. That logic cannot be tested without loading the Plugin Framework, and a second caller (a CLI, a test harness, a later provider) would copy it.

## Decision

`internal/orb` and `internal/docker` are the upstream clients. They import neither `terraform-plugin-framework` nor Terraform types.

The provider calls a small surface:

- machines: `CreateMachine`, `GetMachine`, `UpdateMachine`, `RemoveMachine`, `ListMachines`
- containers: `CreateContainer`, `FindContainer`, `UpdateContainer`, `RemoveContainer`, `SpecFromRequest`

`BuildCreateArgs`, `ParseInfo`, `LimitsFromConfig`, and `BuildSpec` are pure functions in those packages. Schema files translate Terraform values into the request structs and copy results back into state.

## Consequences

- Client tests do not boot a provider server.
- A bug in argv or port mapping is fixed in one package.
- The provider stays boring on purpose. Lifecycle rules that are really Terraform concerns (`RequiresReplace`, preserving an image reference the API rewrites) stay in `internal/provider`.
- Anyone adding a resource starts in the client. A new `orbctl` flag does not begin life as a schema attribute with a shell call beside it.
