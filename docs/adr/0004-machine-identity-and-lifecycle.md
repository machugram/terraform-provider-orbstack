# ADR 0004: Identify machines by ULID and replace only immutable fields

Date: 2026-09-30
Status: Accepted

## Context

`orbctl info -f json` returns a ULID in `record.id` and a separate `name`. `orbctl rename` keeps the ULID. Create-time flags (`image`, architecture, user, cloud-init, isolation, mounts) are not editable later. CPU, memory, and disk are `orbctl config set` keys and can change after create. Power and the default machine are separate commands.

Using the name as the Terraform id makes a rename look like destroy and create, and it breaks import after a rename done outside Terraform.

OrbStack stores an unlimited CPU, memory, or disk limit as `0`. Writing `0` into state would fight a practitioner who left the attribute unset.

## Decision

The Terraform id is `record.id`. `name` updates in place via `orbctl rename`.

These attributes force replace: `image`, `arch`, `username`, `cloud_init`, `cloud_init_file`, `isolated`, `isolate_network`, `mounts`. `cloud_init` and `cloud_init_file` conflict. Non-empty `mounts` require `isolated = true`.

`cpus`, `memory_mib`, and `disk_gib` update through `machine.<name>.cpu`, `machine.<name>.memory_mib`, and `machine.<name>.disk_bytes`. Disk is stored as GiB times `1024^3`. If the machine is running, the client restarts it so the limit applies. A `0` from OrbStack becomes null in the client before the provider sees it.

`power_state` maps to `start` and `stop`. `default_machine` maps to `orbctl default`. Clearing it runs `orbctl default none` only when this machine is the current default.

`forward_ssh_agent` is omitted. The create flag only enables it, and info already reports it true.

Read keeps the configured image string when state already has one. OrbStack expands `ubuntu` to a distro and version. Writing that expansion back would replace the machine on every refresh.

## Consequences

- Rename and import by ULID agree with OrbStack's identity.
- Practitioners can see which edits are in place and which destroy the VM. That is the plan, not a surprise at apply.
- Unlimited limits stay null, so an unset `cpus` does not plan a change when OrbStack reports `0`.
- Cloud-init cannot be read back. Import leaves it empty. The next plan replaces the machine if configuration introduces cloud-init, which is honest: OrbStack applied that data only at create.
- Restart-on-limit-change briefly stops a running machine. Skipping the restart would leave the documented limit different from the running one.

## Options considered

- Name as id. Simpler imports, and rename becomes destroy/create plus a forced new ULID.
- Treat every machine field as `RequiresReplace`. Easier code, and a one-core change would wipe the disk.
