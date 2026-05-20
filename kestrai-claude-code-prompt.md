# Claude Code Prompt: Build Kestrai — an Open-Source AI Agent Orchestrator

> Paste this entire file as your opening message to Claude Code. It is intentionally detailed so Claude Code doesn't waste cycles on design churn. Anything marked **OPEN QUESTION** should be confirmed with the user before coding starts.

---

## 1. Mission

Build the **"Kubernetes of AI agents"**: an open-source, declarative orchestrator that takes a high-level spec and coordinates a fleet of specialized agents to drive real work to completion. A user writes a YAML spec describing the goal — software to ship, a cluster to operate, a runbook to execute — and Kestrai launches a graph of agents that plan, act through gated tools, observe results, and converge the system. Two domains are first-class in v1:

- **Software builds** — refine requirements, plan, design, code, test, review, ship.
- **DevOps / SRE** — analyze infrastructure, author and apply manifests, orchestrate deploys, triage incidents, execute runbooks against live clusters and instances.

The core (resources, reconciler, scheduler, tool gateway, model router, event log) is identical for both. What differs is the **bundle** — which Agents and Tools are loaded. Bundles ship as plugins; the core ships none.

Project name: **Kestrai**. CLI: **`kestrai`**. License: **Apache 2.0**.

---

## 2. Non-Negotiable Principles

1. **Declarative-first.** Every resource is a spec. A control plane reconciles desired state into actual state. No imperative "do X then Y" scripts in the core flow.
2. **Everything pluggable.** Models, agents, tools, storage, auth, telemetry, notifiers, guardrails — all behind interfaces with reference implementations. If a feature can't be swapped, it's not done.
3. **Event-sourced.** All state transitions are events on an append-only log. State stores are projections of the log. This gives replay, audit, and time-travel debugging.
4. **Polyglot SDK, single control plane.** Control plane in one language (Go). Agent SDK available in multiple languages, Python first.
5. **Local-first dev.** `kestrai up` runs the full stack on a laptop with SQLite. Zero required cloud services for development.
6. **Observable by default.** OpenTelemetry traces and structured logs for every agent call, model invocation, and tool use. No black boxes.
7. **Safe by default.** Agents run in sandboxes (containers minimum). Capability-based permissions: agents declare what tools/data they need. Secrets never reach agent process memory unless explicitly granted.
8. **Boring tech wins.** Prefer Postgres over a new vector DB, Cobra over a hand-rolled CLI framework, etc. Save innovation budget for the agent layer.

---

## 3. Architecture Overview

### Control Plane
- **API Server** — gRPC + REST gateway, OpenAPI spec generated from protobuf.
- **Reconciler** — Kubernetes-style controller loop, one per resource type. Watches desired vs actual state, drives convergence.
- **Scheduler** — Assigns tasks to agent runtimes based on model availability, cost, latency, locality, and policy.
- **State Store** — Pluggable: SQLite (dev), Postgres (prod). Event log on NATS JetStream.
- **Plugin Manager** — Loads gRPC plugins, WASM modules, webhook integrations.
- **Auth Service** — OIDC, API keys, RBAC.

### Data Plane
- **Agent Runtime** — Long-lived workers that execute agent specs. Run in containers; spawn ephemeral sub-processes per task. Tasks may be long-running (minutes to hours) with streamed output and periodic heartbeats — `kubectl logs -f`, `terraform apply`, multi-step builds are all in scope.
- **Tool Gateway** — Mediates *all* tool calls (file system, shell, web, third-party APIs, **infrastructure: kubectl/SSH/cloud SDKs/Terraform**) with permission checks and audit. Every external action an agent takes goes through here; there is no second path.
- **Model Router** — Selects model per task, handles fallback, retries, rate limits, cost tracking. Maintains per-provider quotas.

### Interfaces
- **CLI: `kestrai`** — Go, Cobra. Kubectl-style verbs: `apply`, `get`, `describe`, `delete`, `logs`, `exec`, `port-forward`, `top`, `explain`. Single static binary.
- **Web UI** — Next.js 15 + React Server Components, Tailwind + shadcn/ui. Views: workflow graph (React Flow), run timeline, agent inspector, log/trace viewer, kanban-style task board, cost dashboard.
- **TUI** — Go + Bubble Tea, k9s-style live monitor.
- **Python SDK** — `from kestrai import Agent, Workflow, Tool`. Pydantic models for resources.
- **TypeScript SDK** — Parity with Python (Phase 2+).
- **Webhook API** — GitHub PR opened → trigger Workflow, Linear ticket assigned → trigger Workflow, etc.

### Core Resources (CRD-equivalents)
| Resource | Purpose |
|---|---|
| `Project` | Top-level container: settings, secret refs, default model, budget cap |
| `Workflow` | DAG of agent tasks (a "Deployment" analogue) |
| `Agent` | An agent type: prompt template, model preferences, allowed tools, sub-agent policy |
| `Task` | A unit of work assigned to an agent |
| `Run` | A single execution of a Workflow with status, events, artifacts |
| `Plugin` | Addon registration |
| `ModelProvider` | LLM provider config (Anthropic, OpenAI, Google, Ollama, …) |
| `Tool` | Tool definition (MCP server, OpenAPI spec, shell, custom gRPC) |
| `Secret` | Encrypted credential reference |
| `Policy` | Permissions, budgets, content guardrails |

All resources versioned `v1alpha1` → `v1beta1` → `v1` with a clearly documented stability promise.

---

## 4. Default Agent Bundles

Bundles are sets of Agent + Tool definitions distributed together — analogous to a Helm chart or a Linux distro's package group. The **core ships no bundles**; reference bundles ship as plugins on the same release cadence as Kestrai itself.

Each name below is a **role**, not an implementation. Every one is swappable via the `Agent` resource. Users may mix-and-match bundles (a DevOps run can summon `Architect` from the software bundle; a software run can summon `LogAnalyst` from the DevOps bundle) — bundles are not silos.

### 4.1 Software-Build Bundle (ships Phase 2)

The reference flow demoed in Phase 2: spec → shipped code.

**Inception phase**
- `SpecRefiner` — clarifies ambiguity, asks user for missing info
- `RequirementsAnalyst` — extracts functional and non-functional requirements
- `DomainResearcher` — investigates the problem domain
- `StackInvestigator` — analyzes existing code or recommends a stack

**Planning phase**
- `Architect` — high-level system design
- `Planner` — breaks work into epics, milestones, sprints
- `RiskAssessor` — identifies risks, dependencies, unknowns
- `ModelRouter` — chooses optimal model per task (cost/quality/latency)
- `SubAgentStrategist` — decides what to parallelize and into how many sub-agents

**Execution phase**
- `Coder` — instantiated *per workstream* (one per service/module), not one mega-agent
- `FrontendDesigner` — UI/UX design and component implementation
- `SchemaDesigner` — database/schema design
- `InfraEngineer` — IaC, CI/CD, deployment config
- `DocsWriter` — README, API docs, user docs

**Review phase**
- `CodeReviewer` — code review on PRs
- `SecurityReviewer` — security review (SAST-style + LLM judgment)
- `TestWriter` — writes unit, integration, regression, perf, e2e tests (one agent, multiple strategies)
- `PerfTester` — runs the perf tests and analyzes results
- `AccessibilityAuditor` — a11y review
- `UxReviewer` — UX critique

**Meta**
- `PlanReviewer` — reviews the plan itself before execution
- `Retrospective` — post-completion analysis, feeds learnings back
- `MemoryManager` — manages long-term context across the run (vector store + summarization)

**Notes / deviations from the user's list:**
- Regression, integration, and perf tests are folded into `TestWriter` with strategies, not separate agents — same skill, different config.
- Added `MemoryManager` — without it, long workflows lose context catastrophically.
- Added `PlanReviewer` — the plan itself is the highest-leverage artifact to review.
- `Coder` is one type instantiated many times, not one giant agent.

### 4.2 DevOps / SRE Bundle (ships Phase 3 alongside the kubernetes / terraform / ssh tool plugins)

The reference flow for operating real infrastructure: intent → manifests → applied state, plus incident response and runbook execution. Same orchestrator, different agents and tools.

**Inspection phase**
- `ClusterInspector` — read-only cluster state survey (workloads, events, resource pressure, recent changes)
- `LogAnalyst` — log triage, pattern detection across services
- `DriftDetector` — compares declared state (Git) to actual state, flags drift with blast-radius assessment

**Planning phase**
- `ArchitectSRE` — high-level infra design (mirrors `Architect` but for systems, not software)
- `ChangePlanner` — sequences a change into safe steps with rollback points
- `RiskAssessor` — shared with the Software-Build bundle; reused here for blast-radius and impact analysis

**Execution phase**
- `ManifestAuthor` — generates Kubernetes manifests / Terraform / Pulumi from declared intent
- `DeployOrchestrator` — drives a rollout: apply → wait → health-check → next, with explicit gating
- `RunbookExecutor` — runs procedural runbooks step-by-step against live infrastructure
- `IncidentResponder` — gathers context for an active incident (logs, metrics, recent deploys, similar past incidents)

**Review phase**
- `ManifestReviewer` — reviews IaC for safety, best practices, security posture, cost impact
- `SecurityReviewer` — shared with the Software-Build bundle
- `PostmortemAuthor` — generates structured postmortems from event logs after incidents close

**Meta**
- `MemoryManager` — shared with the Software-Build bundle
- `RetroSRE` — operational retrospective; feeds learnings into runbook updates

**Hard rules for the DevOps bundle** (codified in the bundle's bundled Policies):
- No agent in this bundle gets default write access to production. Production write tools require explicit `Policy` grants per-environment and per-action, with `approval: human` on the workflow phase.
- Every `terraform apply` / `kubectl apply` / `ssh` action is recorded as an event with the full intended change and the operator who approved it.
- Long-running tool calls (rollouts, applies, log streams) MUST use the streaming task semantics defined in Phase 1.

---

## 5. Plugin System

Three plugin types, in increasing isolation:

| Type | Mechanism | Use case |
|---|---|---|
| **Native** | Go package compiled in | First-party features |
| **gRPC** | Out-of-process, HashiCorp/go-plugin style | Most third-party extensions |
| **WASM** | Sandboxed (wazero runtime) | Untrusted community plugins |

### Extension Points
- `ModelProvider` — new LLM backends
- `Tool` — new tools agents can use. Reference implementations cover both software (shell, fs, web fetch, MCP servers, OpenAPI) and infrastructure (kubectl, SSH, Terraform, cloud SDKs — see Phase 3)
- `Storage` — state store backends
- `Auth` — identity providers (OIDC, SAML, SSO)
- `Secret` — secret stores (Vault, AWS KMS, GCP Secret Manager, 1Password, …)
- `Telemetry` — exporters (Datadog, Honeycomb, Grafana Cloud, …)
- `Notifier` — Slack, Discord, email, webhooks, PagerDuty
- `Guardrail` — content filters, PII redactors, budget enforcers, prompt-injection detectors, **production-write gates** (block destructive actions outside approved windows)
- `AgentTemplate` — new agent role definitions
- `EncryptionProvider` — at-rest and in-transit encryption strategies
- `Bundle` — a curated set of Agents, Tools, and Policies distributed together (e.g. the DevOps bundle in §4.2)

### Example plugin manifest
```yaml
apiVersion: kestrai.dev/v1alpha1
kind: Plugin
metadata:
  name: anthropic-provider
spec:
  type: grpc
  extensionPoint: ModelProvider
  binary: ./plugins/anthropic
  config:
    apiKeySecretRef: anthropic-api-key
    defaultModel: claude-opus-4-7
```

The encryption-at-rest, SSO, multi-provider, multi-user, and token-budget requirements the user mentioned all map to specific extension points above. None of them belong in the core.

---

## 6. Example User-Facing Spec

```yaml
apiVersion: kestrai.dev/v1alpha1
kind: Workflow
metadata:
  name: build-saas-mvp
  project: book-clubs
spec:
  goal: |
    Build a multi-tenant SaaS app for managing book clubs.
    Users join clubs, schedule meetings, track current reads, vote on next picks.
  constraints:
    stack: [typescript, nextjs, postgres]
    deadline: 2026-06-01
    budget: $50
  pipeline:
    - phase: inception
      agents: [SpecRefiner, RequirementsAnalyst, StackInvestigator]
      approval: human          # pause for human review
    - phase: planning
      agents: [Architect, Planner, ModelRouter, PlanReviewer]
      approval: human
    - phase: execution
      agents: [Coder, FrontendDesigner, SchemaDesigner, TestWriter, DocsWriter]
      parallelism: 4
    - phase: review
      agents: [CodeReviewer, SecurityReviewer, PerfTester, AccessibilityAuditor]
      gating: true             # block on failures
```

CLI parity:
```bash
kestrai apply -f workflow.yaml
kestrai get runs
kestrai describe run/build-saas-mvp-abc123
kestrai logs run/build-saas-mvp-abc123 --agent Coder --follow
kestrai approve run/build-saas-mvp-abc123 --phase planning
```

---

## 7. Tech Stack

- **Control plane**: Go 1.22+, gRPC, Protobuf, sqlc or Bun for SQL, NATS JetStream for event log
- **Agent SDK**: Python 3.12+ (primary, with uv), TypeScript (Phase 2)
- **CLI**: Go, Cobra, Viper
- **GUI**: Next.js 15, React Server Components, Tailwind, shadcn/ui, TanStack Query, React Flow for graphs
- **TUI**: Go, Bubble Tea
- **Testing**: Go test + testify, Pytest, Playwright (GUI e2e), Testcontainers
- **Build/Release**: GoReleaser, Docker (multi-arch), GitHub Actions, Helm chart for K8s
- **Docs**: Docusaurus or Mintlify

---

## 8. Phased Delivery

### Phase 0 — Foundation (skeleton, no real agents yet)
**Goal**: end-to-end spine. Submit a spec, see it scheduled, see a no-op task complete. Hobbyist DX bar (Section 12 item 6) already met.

Deliverables:
- Monorepo layout (Go workspaces + Python uv workspaces + pnpm workspaces), all repo-meta files from Section 11
- Protobuf definitions for core resources, including `tenantId` on every tenant-scoped resource
- API server with health, version, resource CRUD for `Project` and `Workflow`
- `AuthProvider` interface defined, with `local-dev` (no-auth) and `static-token` reference implementations
- SQLite state store with migrations; every tenant-scoped table has a `tenant_id` column with a `default` row seeded
- Reconciler skeleton with `Workflow` fully wired (desired → actual state convergence)
- `kestrai up` — all-in-one process (API + reconciler + scheduler + embedded NATS + SQLite + local agent runtime via goroutines), starts in under 5 seconds
- `kestrai server <role>` — same binary, single-role process mode (stub OK for Phase 0, real wiring in Phase 1)
- `kestrai apply / get / describe / delete / logs / version / doctor`
- `kestrai init` — scaffolds a starter workflow in the current directory
- "Hello World" agent that just logs its input and completes
- `examples/` directory with at least one working `hello-workflow.yaml`
- End-to-end test: `kestrai up` → `kestrai apply -f examples/hello-workflow.yaml` → Run reaches `Succeeded` in <30s
- Every error returned by the API and CLI carries a stable error code; `kestrai explain <code>` prints the doc entry
- CI: lint, test, build for Linux/macOS (amd64 + arm64), Apache 2.0 header check, DCO sign-off check, link-check on docs

### Phase 1 — Core Orchestration
**Goal**: real workflow execution with pluggable models, including long-running tool calls.

Deliverables:
- All core CRDs implemented and reconciled
- Workflow DAG executor with parallelism and phase gates
- Model router with Anthropic + OpenAI + Ollama providers
- Python agent SDK with one reference agent (`SpecRefiner`)
- Tool gateway with shell, file system, web fetch — all sandboxed
- **Long-running task semantics**: a `Task` may run for hours. The Tool Gateway streams stdout/stderr as events on `kestrai.events.<tenant>.tasks.<task-id>.output`. Agents heartbeat every 30s; missed heartbeats past a grace window mark the task `Stalled`. Tasks may extend their deadline via a `RequestDeadlineExtension` event subject to Policy. The CLI's `kestrai logs --follow` tails the stream.
- **`process-exec` tool** — declaratively wrapped CLI invocations (a `Tool` resource points at a binary on PATH with an arg allowlist, env allowlist, working-dir constraint, and timeout). This is the generic primitive every infra plugin in Phase 3 builds on; it also lets users wire arbitrary CLIs (`gh`, `psql`, `helm`, `ansible`) without writing a plugin.
- Event log with replay command (`kestrai replay run/<id>`)
- Structured logs + OpenTelemetry traces with `tenantId` on every span
- TUI for live workflow monitoring
- Docker Compose for local Postgres + NATS (for users who want to test the split-process mode)
- `kestrai server <role>` fully wired so the same code runs all-in-one *or* split into separate processes

### Phase 2 — Default Agent Pipeline + GUI
**Goal**: a user can go from spec to working code with the bundled agents.

Deliverables:
- Default agent roster fully implemented (Section 4)
- Web UI: workflow graph, run timeline, agent inspector, log viewer, kanban board, cost dashboard
- `MemoryManager` with pgvector default backend
- Human-in-the-loop approval gates
- Cost tracking and budget enforcement
- Two end-to-end demos: (a) build a small TypeScript CLI, (b) build a small FastAPI service

### Phase 3 — Plugin Ecosystem (+ DevOps bundle)
**Goal**: extensibility is real, not theoretical. DevOps becomes a first-class user story.

Deliverables:
- gRPC plugin SDK with template repo (`kestrai-plugin-template`)
- WASM plugin runtime via wazero
- `kestrai plugins list/install/remove`
- **Reference infrastructure plugins** (the DevOps bundle's tool surface):
  - `kestrai-plugin-kubernetes` — client-go-backed Tools: `k8s.apply`, `k8s.get`, `k8s.rollout-status`, `k8s.logs-stream`, `k8s.exec`, `k8s.delete` (gated). Each Tool declares its required RBAC verbs.
  - `kestrai-plugin-terraform` — Tools: `terraform.plan`, `terraform.apply` (always behind `approval: human`), `terraform.show`, `terraform.state-list`. Plan output is structured and indexable, not just raw text.
  - `kestrai-plugin-ssh` — Tools: `ssh.exec`, `ssh.stream`. Host allowlist + bastion support + known_hosts pinning.
  - `kestrai-plugin-aws` / `gcp` / `azure` — typed Tool surfaces over each cloud's SDK (read-only by default; writes require explicit Policy grants).
- **Reference observability/notifier plugins**: Datadog telemetry, HashiCorp Vault secrets, Linear notifier, Slack notifier, PagerDuty notifier
- **DevOps Bundle release**: the §4.2 agent roster, packaged with the infra plugins above, plus 3 worked examples (Kubernetes rolling deploy with health checks; Terraform plan-review-apply with human gate; on-call incident triage)
- Webhook ingress (GitHub PR opened → trigger workflow; Alertmanager firing → trigger incident workflow)
- Plugin docs, contributor guide, examples

### Phase 4 — Enterprise & Multi-tenant
**Goal**: production-grade for teams. Note: for the DevOps bundle, several of these items are not "nice-to-have enterprise polish" — they are **adoption prerequisites**. Running an agent against a live cluster without RBAC, mTLS between control plane and data plane, and exportable audit logs is not responsible. Treat the items marked **[DevOps-blocker]** as gating for the DevOps bundle's `v1beta1` promotion, not optional.

Deliverables:
- OIDC + SAML SSO
- RBAC with project-scoped roles **[DevOps-blocker]**
- Multi-tenant data isolation
- Encryption at rest (envelope encryption, KMS-backed)
- mTLS between control plane and data plane **[DevOps-blocker]**
- Audit log export **[DevOps-blocker]**
- Helm chart for production K8s deploy
- HA mode for control plane (active-active with NATS clustering)
- Token + cost budgets per user / team / project
- SOC 2 prep: access logs, retention policies, key rotation

---

## 9. Quality Bar

- Test coverage: ≥80% line, ≥70% branch for control plane
- Every public API: integration tests
- Every CLI command: at least one e2e test
- Every resource type: reconciler tests with desired/actual state fixtures
- Docs site builds on every PR; broken links fail CI
- All errors typed (Go), with stable error codes, documented
- API stability: `v1alpha1` may change freely; `v1beta1` requires deprecation period; `v1` is forever
- Performance: scheduler handles 10k pending tasks with <100ms p99 enqueue latency
- Reproducibility: every Run replayable from its event log to produce identical artifacts (modulo LLM non-determinism, which is logged with seeds where available)
- Security: every external input sanitized; threat model document for each phase

---

## 10. Anti-Patterns to Avoid

- **God-agents.** No 5000-token mega-prompt that does everything. One agent, one role.
- **Hidden state.** Nothing important lives in agent memory only — all decisions go through events.
- **Lock-in to one provider.** Never hardcode Anthropic, OpenAI, etc. Always route through `ModelProvider`.
- **Synchronous chains.** Don't `agent1.run().then(agent2.run())`. Submit tasks to the scheduler.
- **Magic.** Every action an agent takes appears in the trace and traces back to a spec line.
- **Premature plugin abstractions.** Phase 0 has no plugin system. Don't build it before the core works.
- **Reinventing K8s primitives badly.** Where K8s conventions exist (labels, selectors, namespaces, finalizers, watch streams), copy them faithfully.

---

## 11. Decisions Already Made

These are locked in. Do not re-litigate; raise a concern only if implementation reveals a hard blocker.

- **Name:** Kestrai. CLI binary: `kestrai`. API group: `kestrai.dev`.
- **License:** Apache 2.0. Every source file gets the standard Apache header; ship `LICENSE` and `NOTICE` in the repo root; CI enforces headers.
- **Initial target user:** Hobbyists and OSS contributors. The day-one DX target: a developer installs one binary and has their first workflow running in under 10 minutes. Enterprise features (SSO, RBAC enforcement, audit export, HA) are *not* in v1, but the architecture must not preclude them — see Section 12.
- **Hosting model:** Designed to be hostable later. The control plane is built with multi-tenancy as a first-class concept from day one, even though v1 only exercises single-tenant mode. Tenancy resolves automatically to `default` in local-dev so hobbyists never see it.
- **Repo structure:** Monorepo. Go workspaces + Python uv workspaces + pnpm workspaces. One CI pipeline, language-scoped jobs.
- **Bundled models:** None. Bring-your-own API key. `kestrai up` does *not* spawn Ollama. Document how to wire Ollama as a `ModelProvider` for users who want fully-local.
- **Agent SDK languages:** Python only for v1. TypeScript SDK deferred to Phase 2. The agent ↔ control-plane wire protocol *must* be language-neutral (gRPC + protobuf, no pickle, no Python-only RPC) so TS can be added without protocol changes.
- **Governance:** BDFL with documented graduation criteria. Use DCO (`Signed-off-by` commits), **not** a CLA.
  - `GOVERNANCE.md` states: BDFL retains final say until **≥5 active maintainers from ≥3 organizations have each contributed for ≥6 months**, at which point governance transitions to a maintainer council (majority vote on technical decisions, BDFL retains tiebreak for one additional year).
  - Foundation move (CNCF / LF AI & Data) is explicitly *not* a v1 decision.
- **Required day-one repo files:** `LICENSE`, `NOTICE`, `README.md`, `CONTRIBUTING.md` (incl. DCO instructions), `SECURITY.md`, `GOVERNANCE.md`, `ARCHITECTURE.md`.

## 12. Design Tensions to Hold Simultaneously

Hobbyist DX wants *"one binary, zero config, just works."* Hostable design wants *"tenancy, auth, and isolation abstractions everywhere."* These pull in opposite directions. Resolve them as follows — these are not suggestions, they are constraints:

1. **Tenancy is in the schema; invisible in the CLI by default.** Every resource has a `tenantId` column from day one. In local-dev mode it auto-fills to `default`. The CLI does not require or expose `--tenant` flags in v1. (Pattern: Kubernetes `default` namespace.)

2. **Auth is an interface with a no-auth local-dev implementation.** Define the `AuthProvider` interface in Phase 0 with two reference providers: `local-dev` (single user, no token required) and `static-token` (single shared bearer token, for self-hosted multi-user). OIDC and SAML arrive in Phase 4 as plugins. The interface must support all three from the start so retrofitting isn't needed.

3. **One binary, multiple roles.** The `kestrai` binary runs in two modes:
   - `kestrai up` — all-in-one process: API server + reconciler + scheduler + embedded NATS + SQLite + a local agent runtime, all in one Go process via goroutines. For hobbyists; starts in under 5 seconds with no config.
   - `kestrai server <role>` — single-role process (`api`, `reconciler`, `scheduler`, `agent-runtime`) talking over gRPC, external NATS, external Postgres. For self-hosters and future hosting providers.
   - Same code paths; only process boundaries differ. (Pattern: Consul, Vault, Nomad.)

4. **Instrumentation is per-tenant from day one.** Cost tracking records `(tenantId, projectId, runId, modelProvider, tokens, costUsd, latencyMs)` from Phase 2 onward. Audit log entries carry `tenantId`. Anyone hosting later has the data they need to build billing without schema migrations.

5. **No premature hosting machinery.** Do *not* build: billing, signup flows, multi-region replication, hosted dashboards, tenant provisioning APIs. The line is: *schema and interfaces ready, no business logic.*

6. **Hobbyist DX bar — Phase 0 and 1 must meet all of these:**
   - `kestrai up` runs in under 5 seconds on a developer laptop with no flags
   - `kestrai init` scaffolds a working starter workflow
   - First workflow runs end-to-end in under 10 minutes from `git clone` to green
   - Every error includes either an actionable fix or a `kestrai explain <error-code>` reference
   - The CLI has colorized, progressive output; no walls of JSON unless `--output json`
   - At least 5 working example workflows in `examples/` — at least one **software-build** and at least one **DevOps** (e.g. a Kubernetes rolling deploy or a Terraform plan-review-apply runbook). The bundles ship later, but the workflow YAML shape must be stable enough that the examples don't need to be rewritten.
   - `kestrai doctor` diagnoses common local setup problems

7. **General-purpose core, opinionated bundles.** The core (resources, reconciler, scheduler, tool gateway, model router, event log) is domain-neutral. The opinionated parts — *which* Agents exist, *which* Tools are wired up, *which* Policies are enforced — live in **bundles** distributed as plugins (§4, §5, §8/Phase 3). v1 ships two reference bundles: Software-Build (Phase 2) and DevOps/SRE (Phase 3). Avoid the pull to bake either bundle's vocabulary into the core; if a feature only makes sense for one bundle, it belongs in that bundle's plugin, not in the API.

---

## 13. Working Style

- Make a plan, share it, refine, then build.
- For each phase, scaffold first (file layout, interfaces, stubs), then fill in.
- One PR per logical change. Keep PRs under ~500 lines when possible.
- Tests written alongside the feature, not at the end.
- Docs updated in the same PR as the feature.
- "Make it work, make it right, make it fast" — in that order.
- When in doubt, choose boring tech.

---

## 14. Deliverable for the First Session

Deliver **Phase 0 in full**, ending with:
- A working flow: `kestrai up` (terminal 1) + `kestrai apply -f examples/hello-workflow.yaml` (terminal 2) → Run reaches `Succeeded`
- All Phase 0 deliverables checked off, including the hobbyist DX bar (Section 12 item 6)
- `ARCHITECTURE.md` explaining the layout, the reconciler flow, and the all-in-one vs split-process modes
- A short demo script (~10 lines) describing what a viewer would see

All product-level questions have been resolved (Sections 11 and 12). If an *implementation* question arises that the prompt doesn't answer — e.g., a specific protobuf field design, a choice between two equivalent libraries — make the call yourself, document the rationale in `ARCHITECTURE.md`, and proceed. Only stop if you hit a hard contradiction between two locked decisions.

---

## 15. References Worth Reading Before Starting

- Kubernetes API conventions: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md
- HashiCorp go-plugin design: https://github.com/hashicorp/go-plugin
- Operator pattern: https://kubernetes.io/docs/concepts/extend-kubernetes/operator/
- Event sourcing fundamentals (Martin Fowler)
- NATS JetStream concepts
- OpenTelemetry semantic conventions for GenAI

---

*End of prompt. Begin by acknowledging this prompt, confirming the open questions, and proposing a concrete Phase 0 file layout for review.*
