# Contributing to Kestrai

Thanks for your interest in helping build Kestrai. This guide covers the dev setup, the conventions we follow, and how to land your first patch.

## Code of conduct

This project follows the [Contributor Covenant 2.1](./CODE_OF_CONDUCT.md). By participating, you agree to uphold it.

## Developer Certificate of Origin (DCO)

We use the [DCO](https://developercertificate.org/) instead of a CLA. Every commit must include a `Signed-off-by` line:

```
git commit -s -m "your message"
```

This adds:

```
Signed-off-by: Your Name <you@example.com>
```

CI rejects PRs whose commits are not signed off. Use `git commit --amend -s` or `git rebase --signoff` to fix unsigned commits.

## Dev setup

Kestrai is a monorepo with three language workspaces:

| Workspace | Tool | Path |
|---|---|---|
| Go (control plane, CLI, TUI) | Go workspaces | `go.work`, `cmd/`, `internal/`, `pkg/` |
| Python (agent SDK) | `uv` | `sdk/python/` |
| TypeScript (Phase 2 GUI + SDK) | pnpm | `web/`, `sdk/typescript/` |

### Prerequisites

- Go 1.22+
- Python 3.12+ and `uv` (`curl -LsSf https://astral.sh/uv/install.sh | sh`)
- Node 20+ and pnpm 9+ (`corepack enable && corepack prepare pnpm@latest --activate`)
- `protoc` (only needed if you change `.proto` files)
- `make` (for the dev shortcuts)

### Bootstrap

```bash
# Sync all workspaces
make setup

# Build the kestrai binary
make build

# Run the all-in-one stack
./bin/kestrai up
```

In another terminal:

```bash
./bin/kestrai apply -f examples/hello-workflow.yaml
./bin/kestrai get runs
```

### Running tests

```bash
make test            # all languages
make test-go         # Go only
make test-python     # Python only
make test-e2e        # full end-to-end
```

To run a single Go test:

```bash
go test ./internal/reconciler/... -run TestWorkflowReconciler_Reconcile
```

To run a single Python test:

```bash
uv run pytest sdk/python/tests/test_agent.py::test_lifecycle
```

### Regenerating protobuf stubs

```bash
make proto
```

## Conventions

- **One agent, one role.** No mega-prompts. (See [ARCHITECTURE.md](./ARCHITECTURE.md) §Anti-patterns.)
- **Every state transition is an event.** No hidden state in agent memory.
- **Never hardcode a model provider** — always route through the `ModelProvider` interface.
- **Tenancy is in the schema from day one.** Every tenant-scoped resource has a `tenant_id` column. The CLI defaults it to `default` and never exposes a `--tenant` flag in v1.
- **Stable error codes.** Every user-facing error has a `KE-xxxx` code documented in [docs/errors/](./docs/errors/) and printable via `kestrai explain <code>`.
- **One PR per logical change.** Aim for <500 lines. Split big features.
- **Tests live with the feature** in the same PR.
- **Docs update in the same PR** as the feature they describe.
- **Apache 2.0 header** on every source file. CI enforces this.

## PR checklist

Before opening a PR:

- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] Each commit is signed off (`git commit -s`)
- [ ] New errors have `KE-xxxx` codes and a `docs/errors/KE-xxxx.md` entry
- [ ] New CRDs have reconciler tests with desired/actual fixtures
- [ ] Docs touched if user-visible behavior changed

## Where to start

- Issues labeled [`good first issue`](https://github.com/kestrai/kestrai/labels/good%20first%20issue) are scoped for newcomers.
- Issues labeled [`help wanted`](https://github.com/kestrai/kestrai/labels/help%20wanted) are larger but have a clear shape.
- New plugins are always welcome — see [docs/plugins/authoring.md](./docs/plugins/authoring.md).

## Reporting security issues

Do **not** open a public issue for security problems. See [SECURITY.md](./SECURITY.md).
