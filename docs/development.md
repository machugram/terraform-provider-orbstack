# Development

## Tests

```bash
go test ./...
```

Tests live next to the clients, not in the provider:

- `internal/orb` checks create argv, `info` JSON, and the rule that a `0` limit is null.
- `internal/docker` checks the mapping onto Docker `Config` and `HostConfig`, including a port outside 1–65535.

No test starts OrbStack, pulls an image, or opens the Docker socket. That split is [ADR 0005](adr/0005-offline-tests.md).

## Local Terraform

Build a binary and point Terraform at its directory. The dev override replaces the provider for every configuration on this machine until you remove it.

```bash
go build -o "$PWD/dist/terraform-provider-orbstack" .
```

`~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "machugram/orbstack" = "/absolute/path/to/terraform-provider-orbstack/dist"
  }
  direct {}
}
```

Then, in `examples/machine` or `examples/container`:

```bash
terraform plan
```

`terraform init` is not required while a dev override is set. Terraform will say the override is in effect.

## Registry docs

`tfplugindocs` writes the pages the Terraform Registry renders. It reads schema descriptions and the files under `examples/provider`, `examples/resources`, and `examples/data-sources`.

```bash
go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@v0.24.0 generate --provider-name orbstack
```

Commit `docs/index.md`, `docs/resources/`, and `docs/data-sources/`, then tag a release. The registry reads those files from the release tag.

Do not point experiments at machines you care about. `delete` removes the named machine. Container delete removes that container only.

## Debugging the plugin

```bash
dlv exec ./dist/terraform-provider-orbstack -- -debug
```

The process prints a `TF_REATTACH_PROVIDERS` value. Export it in the shell where you run Terraform.

## Release

The Terraform Registry reads a public GitHub release. It does not take the local `dist/` binary.

Pushing a tag such as `v0.1.0` runs `.github/workflows/release.yml`. GoReleaser builds the zip files, writes `SHA256SUMS`, and signs that file with the GPG key stored in the `GPG_PRIVATE_KEY` and `PASSPHRASE` repository secrets. The registry manifest declares protocol `6.0`, which is what the Plugin Framework serves.

The registry will not list the provider until you sign in at [registry.terraform.io](https://registry.terraform.io) with the `machugram` GitHub account, add [docs/registry-public-key.asc](registry-public-key.asc) under Signing Keys, and choose Publish, then this repository.

## Adding a resource

1. Put upstream calls in `internal/orb` or `internal/docker`.
2. Add a resource or data source under `internal/provider` that maps schema to that client.
3. Cover the new request mapping with a unit test that does not need OrbStack.
4. Record the decision in `docs/adr` if it changes a lifecycle rule or an upstream boundary.
