-- Copyright 2026 The Kestrai Authors.
--
-- Licensed under the Apache License, Version 2.0 (the "License");
-- you may not use this file except in compliance with the License.
-- You may obtain a copy of the License at
--
--     http://www.apache.org/licenses/LICENSE-2.0
--
-- Unless required by applicable law or agreed to in writing, software
-- distributed under the License is distributed on an "AS IS" BASIS,
-- WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
-- See the License for the specific language governing permissions and
-- limitations under the License.

-- Initial Kestrai control-plane schema.
--
-- The store is an etcd-style typed key/value store: each resource is persisted
-- as its marshaled protobuf in `data`, with the K8s metadata that queries need
-- lifted into indexed columns. Storing the body as an opaque blob means the
-- freely-changing v1alpha1 proto does not force a schema migration on every
-- field edit (see ADR-003).

-- Tenants. Single-tenant deployments only ever hold the seeded 'default' row;
-- the column exists on every tenant-scoped table from day one so hosting can
-- be layered on later without a migration (§12 tension #1).
CREATE TABLE tenants (
    id         TEXT PRIMARY KEY,
    created_at TEXT NOT NULL
);

INSERT INTO tenants (id, created_at) VALUES ('default', datetime('now'));

-- Resources is the generic object table for every CRD-equivalent (Project,
-- Workflow, …). The natural key mirrors a K8s object reference:
-- (tenant_id, kind, project, name). `project` is '' for tenant-global kinds.
CREATE TABLE resources (
    tenant_id        TEXT    NOT NULL REFERENCES tenants (id),
    kind             TEXT    NOT NULL,
    project          TEXT    NOT NULL DEFAULT '',
    name             TEXT    NOT NULL,

    uid              TEXT    NOT NULL UNIQUE,
    -- Monotonic per-row optimistic-concurrency token. Exposed to clients as a
    -- decimal string in ObjectMeta.resource_version; bumped on every write.
    resource_version INTEGER NOT NULL,
    -- Spec generation. Owned by the caller (bumped only on spec changes), the
    -- store persists whatever it is handed.
    generation       INTEGER NOT NULL,

    labels           TEXT    NOT NULL DEFAULT '{}',
    created_at       TEXT    NOT NULL,
    data             BLOB    NOT NULL,

    PRIMARY KEY (tenant_id, kind, project, name)
);

CREATE INDEX idx_resources_list ON resources (tenant_id, kind, project, name);
