# Architecture

This document is the human-readable companion to [`kestrai-claude-code-prompt.md`](./kestrai-claude-code-prompt.md), which remains the canonical build spec. Read both — this one for orientation, that one for binding decisions and phasing.

## One-paragraph version

Kestrai is a declarative control plane for AI agents. A user submits YAML (`Project`, `Workflow`, `Agent`, `Task`, `Run`, `Plugin`, `ModelProvider`, `Tool`, `Secret`, `Policy`) and the control plane reconciles those specs into actual runs. Agents execute in a sandboxed data plane; all tool calls go through a permission-checked Tool Gateway; all model calls go through a pluggable Model Router. **Every state transition is an event on an append-only log (NATS JetStream).** State stores (SQLite for dev, Postgres for prod) are projections of that log. The same binary runs all-in-one (`kestrai up` — goroutines + SQLite + embedded NATS) or split into single-role processes (`kestrai server <role>` — gRPC + external NATS + external Postgres).

## Control plane (Go)

The control plane is a single Go binary (`cmd/kestrai/`) that can run in either of two shapes:

- **All-in-one (`kestrai up`)** — every component in one process, communicating via in-memory queues and an embedded NATS server. Persistence is SQLite. The local agent runtime executes Python agents as subprocesses. This is the hobbyist mode and must start in under 5 seconds with no flags.
- **Split-role (`kestrai server <role>`)** — the same binary launched as one of `api`, `reconciler`, `scheduler`, or `agent-runtime`. Components talk over gRPC; persistence is Postgres; events go through a stand-alone NATS cluster. Pattern: Consul / Vault / Nomad.

The two shapes share code paths. Only the process boundaries differ.

### Components

| Component | Responsibility |
|---|---|
| API server | gRPC + HTTP gateway. Accepts CRUD on core resources, streams events back. |
| State store | SQLite (dev) or Postgres (prod). Holds the latest projected view of every resource. |
| Event log | NATS JetStream subject `kestrai.events.<tenant>.<resource>`. Append-only, replayable, the source of truth. |
| Reconciler | Watches desired vs actual state for each resource type and drives convergence. Pattern: Kubernetes controller-runtime. |
| Scheduler | Decides which `Task` runs on which agent runtime. Honors `Policy` constraints. |
| Auth provider | Interface with `local-dev` and `static-token` reference implementations. OIDC/SAML are Phase 4 plugins. |

## Data plane (agents)

Agents run as Python processes loaded from the Python SDK (`sdk/python/`). The agent ↔ control-plane wire protocol is gRPC + Protobuf — explicitly language-neutral so a TypeScript SDK can be added in Phase 2 with no protocol changes.

Three things are non-negotiable for agents:

1. **All tool calls go through the Tool Gateway.** No direct file system, shell, or network access. The gateway enforces the agent's declared tool permissions.
2. **All model calls go through the Model Router.** No SDK imports of `anthropic` or `openai` inside agent code. Providers are configured by `ModelProvider` resources.
3. **All state lives on the event log.** Agent memory is allowed for in-run scratchpads only — anything that must survive across runs is an event.

## Core resources

| Resource | Role | Phase |
|---|---|---|
| `Project` | Top-level namespace for related work. | Phase 0 |
| `Workflow` | DAG of `Task`s with phase gates and parallelism. | Phase 0 (CRUD) / Phase 1 (executor) |
| `Agent` | Declarative agent definition: role, allowed tools, model. | Phase 1 |
| `Task` | A single unit of work submitted to an agent. | Phase 1 |
| `Run` | An execution instance of a `Workflow`. | Phase 1 |
| `Plugin` | Out-of-tree extension (tool, model provider, auth provider). | Phase 3 |
| `ModelProvider` | A pluggable LLM backend (Anthropic, OpenAI, Ollama, …). | Phase 1 |
| `Tool` | A capability the Tool Gateway can hand out (shell, fs, web fetch). | Phase 1 |
| `Secret` | Sensitive value referenced by name; never logged. | Phase 1 |
| `Policy` | Constraints: rate limits, cost budgets, allowed tools per agent. | Phase 2 |

All tenant-scoped resources have a `tenant_id` column from day one. In local-dev it auto-fills to `default`. The CLI does not require or expose a `--tenant` flag in v1.

## Repository layout

```
cmd/kestrai/         Main binary entry point (Go).
internal/            Private Go packages (api, reconciler, scheduler, store, …).
pkg/                 Public Go packages intended for import by plugins.
proto/               Protobuf source (kestrai.dev/v1alpha1).
gen/go/              Generated Go from proto. Separate module so churn stays out of the main go.mod.
sdk/python/          Python agent SDK (uv workspace member).
sdk/typescript/      TypeScript SDK (Phase 2).
web/                 Next.js GUI (Phase 2, pnpm workspace member).
examples/            Hand-written workflow YAML files. At least 5 by end of Phase 1.
docs/errors/         One `KE-xxxx.md` per stable error code, surfaced by `kestrai explain`.
scripts/             Build, lint, header-check helpers.
.github/workflows/   CI definitions.
```

## Anti-patterns (flagged in review)

- God-agents with mega-prompts.
- Hidden state in agent memory (must go through events).
- Hardcoding a model provider — always route through `ModelProvider`.
- Synchronous `agent1.run().then(agent2.run())` chains — submit `Task`s to the scheduler.
- Plugin abstractions before the core works. Phase 0 has **no** plugin system.
- Reinventing Kubernetes primitives badly — copy K8s conventions (labels, selectors, namespaces, finalizers, watch streams) faithfully.

## Architecture Decision Records

When an implementation question is not answered by the canonical spec, decide, document the rationale here as a new `## ADR-NNN: short title` section, and proceed. ADRs are append-only; supersede an old one with a new one rather than rewriting it.

### ADR-001: Proto layout for `kestrai.v1alpha1`

**Context.** Phase 0 needs protobuf definitions for `Project` and `Workflow` plus the toolchain to compile them. The canonical spec is silent on file layout, Go package strategy, and how to model K8s-style metadata in proto.

**Decision.**

- **Directory.** `proto/kestrai/v1alpha1/` — one file per resource (`project.proto`, `workflow.proto`) plus `meta.proto` for the shared envelope and cross-resource types (`ObjectMeta`, `Condition`, `ListMeta`, `Budget`). Buf's `PACKAGE_DIRECTORY_MATCH` lint rule is satisfied because the path mirrors the package name.
- **Proto package.** `kestrai.v1alpha1`. Bumps to `v1beta1` / `v1` are new directories and new packages, not in-place edits — `v1alpha1` may break freely.
- **Go package.** `github.com/kestrai/kestrai/gen/go/kestrai/v1alpha1`, generated with `paths=source_relative`, short package name `v1alpha1`. Consumers alias on import (`kestraiv1 "..."/v1alpha1`).
- **Generated module isolation.** `gen/go/` is its own Go module (already wired in `go.work`). `make proto` runs `buf generate` then `cd gen/go && go mod tidy`, so protobuf-runtime churn never touches the main `go.mod`.
- **Envelope shape.** Every resource is `{ ObjectMeta metadata; <Resource>Spec spec; <Resource>Status status; }`. K8s-faithful.
- **No `TypeMeta` in proto.** `apiVersion` and `kind` are intrinsic to the message type on the wire; they only matter at the YAML boundary. The CLI's YAML codec will stamp them on marshal and consume them on unmarshal. Keeping them out of proto avoids a useless nested object in every gRPC payload.
- **`tenant_id` on `ObjectMeta`, not per-resource.** Every tenant-scoped resource inherits it uniformly; there is no way to forget it on a new resource. Tenant-global resources (e.g. `Project` itself, `ModelProvider`, `Plugin`) leave `metadata.project` empty but still carry `tenant_id`.
- **`Budget` in `meta.proto`.** Both `Project` and `Workflow` reference it; putting it in a resource-specific file would create a sibling import. Promotes cleanly to a separate `common.proto` if more cross-resource domain types appear.
- **Buf v2 config.** `buf.yaml` and `buf.gen.yaml` at repo root, both `version: v2`. Lint preset `STANDARD` (which includes `ENUM_VALUE_PREFIX`, `ENUM_ZERO_VALUE_SUFFIX`, `PACKAGE_VERSION_SUFFIX`, etc.); breaking-change rules at `FILE` scope so renames inside a file are caught.
- **No gRPC service definitions yet.** The user-facing request was for resource types and the toolchain. Service definitions (`ProjectService`, `WorkflowService`) land alongside the API server work and will live in `proto/kestrai/v1alpha1/*_service.proto`.

**Consequences.** Adding a new resource is mechanical: create `proto/kestrai/v1alpha1/<name>.proto`, import `meta.proto`, follow the `{metadata, spec, status}` shape, run `make proto`. Tenant scoping is automatic for any resource that embeds `ObjectMeta`. Bumping the API to `v1beta1` is a fork of the directory, not a migration.
