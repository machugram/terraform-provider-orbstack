# Architecture

The provider is a Terraform Plugin Framework server. It maps configuration to two clients and writes their results back into state. It does not shell out, parse CLI text, or call the Docker API itself.

```text
Terraform
    |
    v
internal/provider          schema, validation, plan modifiers, state
    |                \
    v                 v
internal/orb          internal/docker
orbctl JSON           github.com/moby/moby/client
    |                 |
    v                 v
Linux machines        OrbStack Docker engine
```

## internal/orb

`Client` runs `orbctl` and returns stdout, stderr, and the exit error. Higher-level methods are what the provider calls:

- `CreateMachine` builds the `create` argv, writes inline cloud-init to a temp file, sets CPU, memory, and disk, then applies power and the default machine.
- `GetMachine` reads `orbctl info -f json`, then the per-machine limit keys.
- `UpdateMachine` renames, writes changed limits, restarts a running machine after a limit change, then start, stop, or `default`.
- `RemoveMachine` deletes by name. A missing machine is success.
- `ListMachines` reads `orbctl list -f json`.

Machine records are decoded from JSON. A limit value of `0` from OrbStack means unlimited and becomes a nil pointer before it ever reaches Terraform.

## internal/docker

`ResolveHost` picks a Docker host and does not open the socket. Order: `docker_host` if set, then `docker context inspect orbstack`, then `unix://$HOME/.orbstack/run/docker.sock`. The first Engine API call dials. A connection error includes that host.

`BuildSpec` turns a request into a Docker `Config` and `HostConfig`. Ports are TCP on `0.0.0.0`. Volume keys are absolute host paths. `CreateContainer` pulls the image, creates the container, and starts it when `running` is true. `FindContainer` inspects by id, then by name. `UpdateContainer` changes only restart policy and running state. `RemoveContainer` force-removes the container and leaves the image and volumes alone.

## internal/provider

Resources and data sources translate `types.String`, `types.Map`, and friends to the client request structs and back. `RequiresReplace` marks fields the upstream APIs apply only at create time. Read refreshes computed fields and the in-place fields. Create-only fields such as the image reference and cloud-init body stay as the practitioner configured them, because neither API can round-trip those strings faithfully.

## What stays out of the provider

| Concern | Where it lives |
| --- | --- |
| `orbctl` argv and JSON | `internal/orb` |
| Cloud-init temp files | `internal/orb` |
| Limit units (`2048M`, GiB to bytes) | `internal/orb` |
| Image pull and container inspect | `internal/docker` |
| Socket discovery | `internal/docker` |
| Schema, defaults, import | `internal/provider` |
