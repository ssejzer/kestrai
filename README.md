<div align="center">

# Kestrai

**Declarative orchestration for AI agents.**
**The Kubernetes pattern, applied to agents.**

[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Status](https://img.shields.io/badge/status-alpha-orange.svg)](#status)
[![Go Reference](https://pkg.go.dev/badge/github.com/kestrai/kestrai.svg)](https://pkg.go.dev/github.com/kestrai/kestrai)
[![Discord](https://img.shields.io/badge/discord-join-7289da.svg)](https://kestrai.dev/discord)

[Quickstart](#quickstart) · [How it works](#how-it-works) · [What's in the box](#whats-in-the-box) · [Docs](https://kestrai.dev/docs) · [Discord](https://kestrai.dev/discord)

</div>

---

Kestrai is an open-source control plane that turns a YAML spec into real work, completed. You describe a goal — software to ship, a cluster to deploy to, a runbook to execute, an incident to triage — and Kestrai schedules a graph of specialized agents that plan, act through gated tools, observe the results, and converge the system. Pluggable model providers, replayable runs, capability-checked tool access, and the same definitions running on your laptop or a production cluster.

Two domains are first-class in v1:

```yaml
# Software build: spec → shipped code
apiVersion: kestrai.dev/v1alpha1
kind: Workflow
metadata:
  name: book-clubs-mvp
spec:
  goal: |
    Build a multi-tenant SaaS app for managing book clubs.
  constraints:
    stack: [typescript, nextjs, postgres]
    budget: $50
  pipeline:
    - phase: inception
      agents: [SpecRefiner, RequirementsAnalyst, StackInvestigator]
      approval: human
    - phase: planning
      agents: [Architect, Planner, PlanReviewer]
      approval: human
    - phase: execution
      agents: [Coder, FrontendDesigner, SchemaDesigner, TestWriter]
      parallelism: 4
    - phase: review
      agents: [CodeReviewer, SecurityReviewer, PerfTester]
      gating: true
```

```yaml
# DevOps runbook: intent → applied infrastructure
apiVersion: kestrai.dev/v1alpha1
kind: Workflow
metadata:
  name: payments-rolling-deploy
spec:
  goal: Roll out payments-api v2.4.1 to prod with health-gated progression.
  constraints:
    cluster: prod-us-east
    budget: $5
  pipeline:
    - phase: inspect
      agents: [ClusterInspector, DriftDetector]
    - phase: plan
      agents: [ChangePlanner, ManifestReviewer]
      approval: human
    - phase: rollout
      agents: [DeployOrchestrator]
      tools: [k8s.apply, k8s.rollout-status, k8s.logs-stream]
      gating: true
    - phase: verify
      agents: [LogAnalyst]
      gating: true
```

```
$ kestrai apply -f workflow.yaml
workflow.kestrai.dev/book-clubs-mvp created

$ kestrai get runs
NAME                    PHASE       STATUS      AGE     COST
book-clubs-mvp-abc123   execution   Running     12m     $3.14
```

---

## Why Kestrai?

Existing agent frameworks like LangGraph, AutoGen, and CrewAI let you wire agents together in Python. That's great for one-off scripts. It falls short when you want:

- A **persistent control plane** other tools and humans can talk to
- **Replayable, auditable runs** instead of black-box LLM calls
- **Pluggable everything** — model providers, tools, storage, auth, secrets — not hardcoded choices
- A workflow that **survives a process restart** and resumes where it left off
- The same definitions running **on a laptop and on a production cluster**

Kestrai applies the Kubernetes design pattern — declarative specs, reconciliation loops, pluggable resource types, capability-based permissions — to AI agents.

> **Think of Kestrai as the kubectl + control plane for agents.** You don't write the agents in Kestrai. You declare what should happen, and Kestrai runs them.

---

## Quickstart

> Kestrai is **bring-your-own-key**. Set at least one model provider API key before starting. See [Providers](https://kestrai.dev/docs/providers) for the full list.

```bash
# Install (Linux/macOS, single binary, no Docker required)
curl -fsSL https://kestrai.dev/install.sh | sh

# Bring your own model API key
export ANTHROPIC_API_KEY=sk-ant-...

# Start the control plane (one process: API + scheduler + reconciler + agent runtime)
kestrai up
```

In a second terminal:

```bash
# Scaffold a starter workflow
kestrai init hello

# Submit it
kestrai apply -f hello/workflow.yaml

# Watch it run
kestrai logs run/hello --follow

# Inspect spend in real time
kestrai top
```

You should see agents come online, refine the spec, plan the work, and complete a tiny end-to-end run in under a minute. If anything looks wrong, `kestrai doctor` diagnoses common setup issues, and every error includes a code you can pass to `kestrai explain`.

---

## How it works

```
┌─────────────────────────────────────────────────────────────┐
│                    kestrai CLI / Web UI / SDK               │
└──────────────────────────────┬──────────────────────────────┘
                               │ gRPC + REST
┌──────────────────────────────▼──────────────────────────────┐
│                       Control Plane                          │
│  ┌──────────┐  ┌────────────┐  ┌──────────┐  ┌────────────┐ │
│  │   API    │  │ Reconciler │  │Scheduler │  │   Plugin   │ │
│  │  Server  │  │   (loop)   │  │          │  │  Manager   │ │
│  └──────────┘  └────────────┘  └──────────┘  └────────────┘ │
│         │              │              │              │       │
│         └──────────────┼──────────────┘              │       │
│                        ▼                             │       │
│              Event Log (NATS JetStream)              │       │
│              State Store (SQLite / Postgres)         │       │
└────────────────────────┬─────────────────────────────┘       │
                         │                                     │
┌────────────────────────▼─────────────────────────────────────┴┐
│                       Data Plane                               │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐    │
│  │Agent Runtime │    │ Tool Gateway │    │Model Router  │    │
│  │  (sandboxed) │◄──►│ (capability- │◄──►│(any provider)│    │
│  └──────────────┘    │   checked)   │    └──────────────┘    │
│                      └──────────────┘                         │
└──────────────────────────────────────────────────────────────┘
```

**You write specs** describing Projects, Workflows, Agents, Tools, and Policies.
**The reconciler** drives actual state toward your spec, exactly like Kubernetes controllers.
**The scheduler** assigns tasks to agent runtimes based on model availability, cost, latency, and policy.
**Every action** is an event on an append-only log — you can replay any run, inspect any decision, and audit any tool call.

---

## What's in the box

**Two reference bundles** — each a curated set of Agents, Tools, and Policies. The core ships none of these; bundles install as plugins. Mix-and-match: a software workflow can summon `LogAnalyst`; a deploy workflow can summon `Architect`.

**Software-Build bundle** *(ships Phase 2)*

| Phase | Agents |
|---|---|
| Inception | SpecRefiner, RequirementsAnalyst, DomainResearcher, StackInvestigator |
| Planning | Architect, Planner, RiskAssessor, ModelRouter, PlanReviewer |
| Execution | Coder *(per workstream)*, FrontendDesigner, SchemaDesigner, InfraEngineer, DocsWriter |
| Review | CodeReviewer, SecurityReviewer, TestWriter, PerfTester, AccessibilityAuditor, UxReviewer |
| Meta | MemoryManager, Retrospective |

**DevOps / SRE bundle** *(ships Phase 3 with the kubernetes / terraform / ssh tool plugins)*

| Phase | Agents |
|---|---|
| Inspect | ClusterInspector, LogAnalyst, DriftDetector |
| Plan | ArchitectSRE, ChangePlanner, RiskAssessor, ManifestReviewer |
| Execute | ManifestAuthor, DeployOrchestrator, RunbookExecutor, IncidentResponder |
| Review | ManifestReviewer, SecurityReviewer, PostmortemAuthor |
| Meta | MemoryManager, RetroSRE |

Every production-write tool (`terraform apply`, `k8s.delete`, `ssh.exec`) requires an explicit `Policy` grant and a `approval: human` phase gate. There is no default path from "agent has an idea" to "agent mutates prod."

**Pluggable extension points** — every one has at least one reference implementation, and every one is replaceable:

- `ModelProvider` — Anthropic, OpenAI, Google, Ollama, your favorite
- `Tool` — shell, file system, web fetch, MCP servers, kubectl, SSH, Terraform, cloud SDKs, your custom gRPC tool
- `Storage` — SQLite (dev), Postgres (prod), or your own
- `Auth` — local-dev, static token, OIDC/SAML *(via plugin)*
- `Secret` — env vars, Vault, AWS KMS, GCP Secret Manager
- `Telemetry` — OpenTelemetry default, Datadog and Honeycomb exporters
- `Notifier` — Slack, Discord, email, webhooks, PagerDuty
- `Guardrail` — content filters, PII redactors, budget enforcers, production-write gates
- `Bundle` — ship your own curated Agent + Tool + Policy set

See [Plugin Authoring](https://kestrai.dev/docs/plugins) to ship your own.

---

## Use cases

- **End-to-end software builds.** Spec → shipped feature, with humans in the loop where it matters.
- **DevOps / SRE workflows.** Gated deploys with health checks, Terraform plan-review-apply with human approval, on-call incident triage that gathers logs and recent changes, drift detection between Git and live infra, runbook execution against real clusters and instances.
- **Long-running research pipelines.** Multi-day workflows that survive restarts, with replayable history.
- **Self-hosted agent platforms.** Run Kestrai for your team behind your firewall, with your own model providers.
- **Agent experimentation.** Swap models, prompts, and tools without rewriting the orchestration.

---

## Status

Kestrai is **alpha**. The API group is `v1alpha1` and **will change** without deprecation periods before `v1beta1`. Use it to experiment, to file issues, and to contribute. Do not run unattended on production workloads yet.

Current phase: see the [milestones page](https://github.com/kestrai/kestrai/milestones) and [public roadmap](https://kestrai.dev/roadmap).

---

## Documentation

- [Getting Started](https://kestrai.dev/docs/getting-started) — install, first workflow, first agent
- [Concepts](https://kestrai.dev/docs/concepts) — Projects, Workflows, Agents, Tasks, Runs
- [Writing Agents](https://kestrai.dev/docs/agents) — Python SDK guide
- [Plugin Authoring](https://kestrai.dev/docs/plugins) — extend everything
- [Architecture](./ARCHITECTURE.md) — internals, reconciler flow, process modes
- [Self-hosting](https://kestrai.dev/docs/self-hosting) — run Kestrai for your team

---

## Community

- 💬 [Discord](https://kestrai.dev/discord) — daily chat
- 🗣️ [GitHub Discussions](https://github.com/kestrai/kestrai/discussions) — ideas, Q&A, show-and-tell
- 🐛 [GitHub Issues](https://github.com/kestrai/kestrai/issues) — bugs and feature requests
- 🦋 [Bluesky](https://bsky.app/profile/kestrai.dev) — updates

---

## Contributing

We'd love your help. Good first stops:

- Read [CONTRIBUTING.md](./CONTRIBUTING.md) — repo layout, dev setup, conventions
- Find an issue tagged [`good first issue`](https://github.com/kestrai/kestrai/labels/good%20first%20issue)
- Ship a plugin and tell us about it

No CLA, no DCO sign-off, no separate paperwork. Kestrai uses GitHub's **inbound = outbound** default: by opening a pull request, you license your contribution under the same Apache 2.0 license that covers the rest of the codebase. Governance is documented in [GOVERNANCE.md](./GOVERNANCE.md).

---

## License

Apache 2.0. See [LICENSE](./LICENSE) and [NOTICE](./NOTICE).

Kestrai is independent open source. It is not affiliated with Anthropic, OpenAI, Google, or any model provider.
