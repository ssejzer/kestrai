# `@kestrai/sdk` (TypeScript) — deferred to Phase 2

The TypeScript agent SDK is **not** part of the v1 release. Python is the only first-class SDK in v1. This directory exists so the monorepo's pnpm workspace already knows where the package will live; it ships intentionally empty until Phase 2.

If you are looking to write an agent today, use the Python SDK in [`sdk/python/`](../python/).

The agent ↔ control-plane wire protocol is gRPC + Protobuf (see [`proto/`](../../proto/)), so adding this SDK in Phase 2 will not require any protocol changes.
