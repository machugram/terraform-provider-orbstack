# Architecture decision records

These records capture decisions that are expensive to reverse or easy to misunderstand from the code alone. Status is Accepted unless a later record says otherwise.

| ADR | Title |
| --- | --- |
| [0001](0001-client-libraries.md) | Keep upstream clients independent of Terraform |
| [0002](0002-orbctl-and-docker-engine.md) | Use orbctl for machines and the Docker Engine API for containers |
| [0003](0003-container-resource-shape.md) | One container resource with an implicit image pull |
| [0004](0004-machine-identity-and-lifecycle.md) | Identify machines by ULID and replace only immutable fields |
| [0005](0005-offline-tests.md) | Keep go test offline |

Format: context, decision, consequences. Date is the day the decision was written down.
