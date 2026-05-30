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

### ADR-002: DevOps as a first-class v1 use case

**Context.** The canonical spec, as originally written, positioned Kestrai as a software-build orchestrator: §1's mission talked about shipping software, §4's default agent roster was entirely software-dev roles (Coder, FrontendDesigner, TestWriter…), and the Phase 1 Tool gateway only planned for `shell` / `fs` / `web-fetch`. A user question asked whether Kestrai could orchestrate DevOps work — agents connecting to Kubernetes clusters and instances to perform real work. The honest answer was "architecturally yes, but the bundled tools and agent roster are software-only and the security story doesn't land until Phase 4." That's a positioning problem more than an architectural one: the core resources (Project / Workflow / Agent / Task / Run / Tool / Policy) are domain-neutral, but the v1 demos and bundled extensions weren't.

**Decision.** DevOps / SRE is a first-class v1 use case, equal in standing to software builds. The repositioning happens entirely above the core — no new resource types, no fork in the reconciler model — by recognising that "what ships with Kestrai" is **bundles** of Agents + Tools + Policies, and v1 ships two reference bundles (Software-Build in Phase 2, DevOps/SRE in Phase 3). Concrete spec amendments:

- **§1 Mission** rewritten to name both domains explicitly.
- **§4 reframed as "Default Agent Bundles".** The original roster is now §4.1 *Software-Build Bundle*. A new §4.2 defines the *DevOps / SRE Bundle*: `ClusterInspector`, `LogAnalyst`, `DriftDetector`, `ArchitectSRE`, `ChangePlanner`, `ManifestAuthor`, `ManifestReviewer`, `DeployOrchestrator`, `RunbookExecutor`, `IncidentResponder`, `PostmortemAuthor`, `RetroSRE`. Bundles are not silos — a software workflow can summon `LogAnalyst`, a deploy can summon `Architect`.
- **§5 Plugin Extension Points** adds a `Bundle` extension point so bundles are a first-class plugin type, not an informal convention. The `Tool` extension point's examples now name infrastructure tools (kubectl, SSH, Terraform, cloud SDKs) explicitly. `Guardrail` adds "production-write gates" as a named pattern.
- **Phase 1** gains long-running task semantics (streamed stdout/stderr events on `kestrai.events.<tenant>.tasks.<id>.output`, 30s heartbeats, deadline extension via `RequestDeadlineExtension` events subject to Policy) and a generic `process-exec` Tool primitive. The streaming primitive is the foundation every infra Tool builds on; the `process-exec` Tool means users can wire any CLI (`gh`, `helm`, `psql`, `ansible`) declaratively without writing a plugin.
- **Phase 3** explicitly names the reference infrastructure plugins: `kestrai-plugin-kubernetes` (client-go-backed, declares required RBAC verbs per Tool), `kestrai-plugin-terraform` (`apply` always behind `approval: human`, structured plan output), `kestrai-plugin-ssh` (host allowlist, bastion support, known_hosts pinning), `kestrai-plugin-aws|gcp|azure` (read-only by default, writes require explicit Policy grants). Phase 3 also ships the DevOps Bundle release with three worked examples.
- **Phase 4** items related to RBAC, mTLS between control plane and data plane, and audit log export are marked **[DevOps-blocker]** — required for the DevOps bundle's `v1beta1` promotion, not optional polish. They remain in Phase 4 because gating Phase 0/1 on them would break the hobbyist DX bar.
- **§12 design tensions** gains item #7: "General-purpose core, opinionated bundles." If a feature only makes sense for one bundle, it belongs in that bundle's plugin, not in the API.
- **DX bar (item 6)** updated so the ≥5 example workflows must include at least one software-build and at least one DevOps example. The bundles ship later, but the workflow YAML shape must be stable enough that the examples don't get rewritten.

**Alternatives considered.**

- *Positioning only* — update the README, leave the spec untouched. Rejected: leaves DevOps as something to figure out in Phase 3, which means the streaming-task work either gets retrofitted (bad) or DevOps work waits another full phase (worse). The Phase 1 work has to know it's coming.
- *Add a `Runbook` / `Procedure` resource alongside `Workflow`* for procedural one-shot flows ("apply A, wait, restart B"). Rejected: a runbook is just a Workflow where each phase has one agent invoking one Tool; bifurcating the model adds surface area without earning its keep. If real friction shows up in Phase 3 we can revisit.
- *Build the DevOps tooling earlier (Phase 1 or 2)*. Rejected: Phase 0 + 1 are foundation work, and stuffing kubectl/Terraform clients into Phase 1 fights the hobbyist DX bar. The Phase 3 plugin ecosystem is the right home; what Phase 1 owes is the streaming + heartbeats + `process-exec` primitive so Phase 3 doesn't have to re-litigate the foundation.

**Consequences.**

- Phase 1 scope grows by the streaming-task and `process-exec` work. Both are small in code (a few hundred lines) but require care in the protobuf and Tool Gateway interface design — getting them wrong forces a refactor when Phase 3 lands.
- Phase 3 scope grows from "build a plugin SDK" to "build a plugin SDK and ship four reference infrastructure plugins and a DevOps bundle." That phase will likely span two or three releases.
- Phase 4 reframes from "enterprise polish" to "DevOps adoption prerequisites + enterprise polish." Practically: RBAC and mTLS get prioritized within Phase 4 over SOC 2 prep.
- The Workflow proto designed in ADR-001 is unchanged by this decision — its goal/constraints/pipeline shape covers DevOps runbooks as-is. The `process-exec` Tool primitive is a new resource shape that lives under `Tool`, not under `Workflow`, so no proto rework either.

### ADR-003: SQLite state store, schema, and migrations

**Context.** Phase 0 needs a state store for `Project`/`Workflow` CRUD: principle #5 mandates SQLite for local dev, principle #2 mandates a pluggable interface, and §3 names Postgres as the prod target. Principle #3 says state stores are projections of the NATS event log. The canonical spec is silent on driver choice, schema shape, and migration tooling. The store lives at `internal/store/`.

**Decision.**

- **Pluggable `Store` interface, SQLite reference impl.** `store.Store` is the seam (`Create`/`Get`/`List`/`Update`/`Delete`/`Close`); `store.SQLite` is the Phase 0 implementation. Postgres slots in behind the same interface in Phase 1+ without touching callers.
- **Pure-Go driver: `modernc.org/sqlite`.** No CGO, so the static multi-arch binary and `scratch`/`distroless` Docker images the release plan calls for still build with `CGO_ENABLED=0`. `mattn/go-sqlite3` (CGO) was rejected for that reason. **Pinned to v1.44.2** — the newest modernc release whose whole tree (libc, cc, ccgo, x/sys) still declares `go 1.24`; v1.49.0+ require Go 1.25, which would force a second floor bump beyond the 1.24 set in PR #2. Revisit when the floor next moves.
- **etcd-style generic object table.** One `resources` table stores each object as marshaled protobuf in a `data BLOB`, with the metadata queries need (`tenant_id`, `kind`, `project`, `name`, `uid`, `resource_version`, `generation`, `labels`, `created_at`) lifted into columns. Natural key `(tenant_id, kind, project, name)` mirrors a K8s object reference. This faithfully copies how K8s stores objects in etcd, and means the freely-changing `v1alpha1` proto does not trigger a schema migration on every field edit. The store never unmarshals `data` — the API handlers own the proto.
- **Optimistic concurrency in storage, generation owned by the caller.** `resource_version` is a monotonic per-row integer the store bumps on every write (exposed as a decimal string); `Update` rejects a stale version with `ErrConflict`. `generation` (spec-change counter) is persisted verbatim because only the handler knows whether a write touched the spec or just status.
- **Hard delete; soft delete is a body edit.** `Delete` removes the row. Finalizer-aware termination (a non-nil `metadata.deletion_timestamp` while finalizers drain) is modeled by the handler via `Update`, so the store stays oblivious to object internals. The finalizer loop itself arrives with the reconciler.
- **Forward-only embedded migrations.** `migrations/NNNN_*.sql` embedded via `embed.FS`, applied in numeric order inside a transaction, tracked in `schema_migrations`. Idempotent on re-open. A hand-rolled ~80-line migrator instead of `golang-migrate` et al. — boring, dependency-free, and enough for Phase 0. Migrations are append-only once released.
- **Tenancy from day one.** A `tenants` table is seeded with the `default` row; `resources.tenant_id` is a foreign key to it. Single-tenant deployments only ever touch `default`; the column and FK exist so hosting can be layered on without a migration (§12 tension #1).
- **Single writer connection.** `SetMaxOpenConns(1)` + WAL + `busy_timeout`. SQLite serializes writers anyway; one connection sidesteps "database is locked" without retry scaffolding. Acceptable for the embedded dev store; Postgres has real connection pooling.
- **In-memory list filtering/paging.** Label selectors (equality-only in Phase 0) and keyset pagination over `(project, name)` are applied in Go after the row scan. Fine at Phase 0 scale; pushes down to SQL when it matters.

**Consequences.** Adding a resource to the store is free — only the handler's proto marshaling changes. The store is written directly by the API server in Phase 0; when the NATS event log lands with the reconciler, writes move to "append event → project into the store," and this interface becomes the projection target rather than the write path. Equality-only label selectors and in-memory paging are explicit Phase 0 simplifications to revisit under load.

### ADR-004: Reconciler skeleton and the `up` / `server` split

**Context.** Phase 0 calls for a reconciler skeleton with `Workflow` desired→actual convergence, plus `kestrai up` (all-in-one) and `kestrai server <role>` (split process, stub OK). Principle #3 makes the NATS event log the eventual source of truth, but the event log is not a Phase 0 deliverable (it lands in Phase 1 with long-running task semantics). The question is how the reconciler observes work and how much split-process machinery to build now.

**Decision.**

- **Poll-based resync, not event-driven yet.** `internal/reconciler` lists Workflows from the store on a short resync interval (1s) and reconciles each. The poll → reconcile → conditional-write shape is the final controller shape; only the *trigger* changes when the NATS watch lands (Phase 1). This keeps Phase 0 free of embedded NATS while staying faithful to the controller model. Consistent with ADR-003 (store written directly in Phase 0).
- **Status-only convergence.** With no scheduler or agent runtime yet, "actual state" for a Workflow is its validation status: the reconciler stamps `Validated` and `Ready` conditions plus `observed_generation`. Run execution (active_run_count, last_run_ref) is wired when the scheduler arrives. Validation is intentionally minimal (goal present, ≥1 named phase).
- **Idempotent writes.** The reconciler writes back only when the computed status differs from the stored one, and carries forward each condition's `LastTransitionTime` when its boolean state is unchanged — so a converged object produces no `resource_version` churn across resyncs. `ErrConflict`/`ErrNotFound` from a racing API write or delete are swallowed; the next resync reconciles fresh state.
- **Single-tenant fan-out.** Phase 0 reconciles the `default` tenant only. Multi-tenant enumeration needs a tenant-listing call the store does not yet expose; deferred with hosting.
- **`kestrai up` is the real all-in-one.** It opens the SQLite store, wires the local-dev `AuthProvider` interceptor (health/version left public), serves gRPC, runs the reconciler in a goroutine, and shuts down gracefully on SIGINT/SIGTERM. Embedded NATS and the agent runtime named in §8's `up` bullet are deferred to Phase 1 alongside the event log and task execution — `up` today is API + reconciler + store.
- **`kestrai server <role>` is a validated stub.** It recognizes the four roles (`api`, `reconciler`, `scheduler`, `agent-runtime`) and points users at `kestrai up`; real split-process wiring (external NATS + Postgres) is Phase 1, as the spec permits.
- **CLI on Cobra; Viper deferred.** The command tree uses Cobra (locked choice). Config is flags + `KESTRAI_*` env fallbacks for now; Viper is added when the resource verbs need layered config/client-connection settings.

**Consequences.** The reconciler is exercised end-to-end by `kestrai up` and by an integration test driving create→reconcile→status over a real gRPC connection. When the event log lands, `reconcileAll`'s trigger changes from a ticker to a NATS subscription and the store write becomes "append event → project," but the reconcile function and status logic stay put. `up` will grow embedded NATS and the agent runtime; the goroutine wiring already hosts them.
