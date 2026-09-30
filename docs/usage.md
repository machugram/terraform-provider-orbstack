# Usage

```hcl
terraform {
  required_providers {
    orbstack = {
      source = "machugram/orbstack"
    }
  }
}

provider "orbstack" {
  # orb_path    = "/usr/local/bin/orbctl"
  # docker_host = "unix:///Users/you/.orbstack/run/docker.sock"
}
```

`orb_path` defaults to `orbctl` on `PATH`. `docker_host` defaults to the OrbStack context, then `~/.orbstack/run/docker.sock`. Provider configuration stores that string. It does not connect until a container operation runs.

Full examples: [examples/machine](../examples/machine) and [examples/container](../examples/container).

## orbstack_machine

Creates a Linux machine.

```hcl
resource "orbstack_machine" "dev" {
  name       = "dev"
  image      = "ubuntu:noble"
  arch       = "arm64"
  username   = "dev"
  cpus       = 2
  memory_mib = 2048
  disk_gib   = 16

  isolated        = false
  isolate_network = false

  power_state     = "running"
  default_machine = true

  cloud_init = <<-EOF
    #cloud-config
    packages:
      - git
  EOF
}
```

`image` defaults to `ubuntu`. Use `distro` or `distro:version`. `power_state` defaults to `running`. `default_machine` defaults to false.

Changing `image`, `arch`, `username`, `cloud_init`, `cloud_init_file`, `isolated`, `isolate_network`, or `mounts` replaces the machine. `cloud_init` is not stored in state. `cloud_init` and `cloud_init_file` cannot both be set. `name` and `image` must not start with `-`. `mounts` entries are `SOURCE` or `SOURCE:DEST`, and they require `isolated = true`.

`name` renames in place. The Terraform id stays the OrbStack ULID. `cpus`, `memory_mib`, and `disk_gib` update in place. A running machine is restarted so the new limit applies. Omit a limit to leave it unlimited. OrbStack reports unlimited as `0`; state stores that as null.

`power_state` is `running` or `stopped`. `default_machine = true` selects this machine. Setting it back to false clears the default only when this machine is the current one.

Computed attributes: `id`, `state`, `distro`, `version`, `disk_size_bytes`.

```bash
terraform import orbstack_machine.dev ubuntu
terraform import orbstack_machine.dev 01KHVDCW7HTAGRPMB823KADP9B
```

Import accepts a name or a ULID. Cloud-init and mounts are not readable from OrbStack, so they stay empty after import until the configuration sets them.

`forward_ssh_agent` is not an attribute. The CLI flag can only turn it on, and OrbStack already records it as true.

## orbstack_container

Creates a container on the OrbStack Docker engine. Create always pulls `image`.

```hcl
resource "orbstack_container" "web" {
  name    = "web"
  image   = "nginx:latest"
  restart = "unless-stopped"
  running = true

  ports   = { "8080" = "80" }
  env     = { NGINX_HOST = "localhost" }
  volumes = { "/tmp/site" = "/usr/share/nginx/html" }

  # command    = ["nginx", "-g", "daemon off;"]
  # workdir    = "/usr/share/nginx/html"
  # cpus       = 1
  # memory_mib = 128
}
```

`ports` and `volumes` are maps from host to container. Ports are integers from 1 to 65535, TCP, published on `bind_address` (`127.0.0.1` unless set). Volume host and container paths must be absolute and must not contain `:`. Mounts are read-write binds. `env` is a name-to-value map and is not stored in state. An empty map and null are the same. Errors from the provider do not print environment values.

`restart` defaults to `unless-stopped` (`no`, `on-failure`, `always`, `unless-stopped`). `running` defaults to true. Stop waits 10 seconds. Only `restart` and `running` update in place.

Changing `name`, `image`, `ports`, `env`, `volumes`, `command`, `workdir`, `cpus`, or `memory_mib` replaces the container. `command` left unset keeps the image command. A tag that moves on the registry does not replace the container. `image_id` is refreshed from inspect and is not configuration.

Delete removes the container and leaves the image in place. Volumes configured here are bind mounts, so they are not deleted either.

```bash
terraform import orbstack_container.web web
```

Import accepts a name or a container id. Environment, command, and workdir are not reconstructed from the image defaults, so an imported container will not gain a command you did not set.

There is no image, network, or Compose resource. Those belong to a full Docker provider if you need them. This resource is the short path for a container on OrbStack.

## Data sources

```hcl
data "orbstack_machine" "dev" {
  name = "dev"
  # or: id = "01KHVDCW7HTAGRPMB823KADP9B"
}

data "orbstack_machines" "running" {
  running = true
}

data "orbstack_container" "web" {
  name = "web"
}
```

`orbstack_machine` takes exactly one of `name` or `id`. It exports `id`, `name`, `state`, `distro`, `version`, `arch`, and `disk_size_bytes`.

`orbstack_machines` lists machines. `running = true` passes `-r` to `orbctl list`. Each element has `id`, `name`, `distro`, `version`, `arch`, and `state`.

`orbstack_container` reads one container by name. It exports `id`, `image`, `image_id`, and `running`.
