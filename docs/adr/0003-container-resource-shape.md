# ADR 0003: One container resource with an implicit image pull

Date: 2026-09-30
Status: Accepted

## Context

Practitioners asked for a short way to run a container on OrbStack. The usual Terraform Docker provider is a full Engine surface: a separate image resource, nested port blocks, and a socket you configure yourself. That is the right tool when you need networks, builds, and registry auth. It is a lot of boilerplate for nginx on a laptop.

OrbStack is already a Docker Engine. Duplicating that entire provider would be a second implementation of the same API with no local advantage.

## Decision

`orbstack_container` is one resource.

- `image` is a reference string. Create and replace always pull. Read does not pull.
- `ports`, `env`, and `volumes` are maps from host to container. Ports are TCP on `0.0.0.0`. Volumes are absolute host paths, bind-mounted read-write.
- `restart` defaults to `unless-stopped`. `running` defaults to true. Those two update in place. Stop uses a 10 second timeout.
- `name`, `image`, `ports`, `env`, `volumes`, `command`, `workdir`, `cpus`, and `memory_mib` force a new container. Docker cannot apply them to a running container without recreate, and pretending otherwise hides a replace in apply.
- Delete force-removes the container. The image stays. Bind mounts are host paths, so they stay too.
- Empty `env` and null `env` are the same. Diagnostics do not include environment values.
- `image_id` is computed from inspect. A tag that moves upstream updates `image_id` on refresh and does not plan a replace. The configured reference is what the practitioner asked for.

Networks, Compose, builds, healthchecks, and registry auth are out of scope.

## Consequences

- A container is a single block. The cost is a smaller API than a general Docker provider.
- Pull on every create makes apply depend on a registry unless the image is already cached. That matches "I named `nginx:latest` and expect that tag."
- Replacing on port or env changes is predictable. In-place mutation of those fields would diverge from Docker's recreate rules the first time the daemon rejected an update.
- Someone who needs a user-defined network should use a Docker provider pointed at the same socket. This provider will not grow into that.

## Options considered

- An `orbstack_image` resource plus a container that references it. Accurate to Docker's two-step API, and twice the configuration for the common case. Pull remains an implementation detail of create.
- Shell-script provisioners on a machine resource. That ties containers to a Linux VM OrbStack does not require for Docker.
