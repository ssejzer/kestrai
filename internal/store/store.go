// Copyright 2026 The Kestrai Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package store is the control-plane state store: a pluggable, etcd-style
// typed key/value store over the core resources. The SQLite implementation
// here is the dev/local reference (Postgres is the prod target, Phase 1+).
//
// Resources are persisted as opaque marshaled protobuf in Record.Data with the
// metadata that queries need lifted into typed fields. The store is unaware of
// any specific resource shape, so adding a new CRD needs no store change. The
// API handlers (which do know the proto) translate between the wire types and
// Record. Per the event-sourcing principle the store is ultimately a
// projection of the NATS event log; Phase 0 writes it directly until the log
// lands with the reconciler (ADR-003).
package store

import (
	"context"
	"errors"
	"time"
)

// Sentinel errors. The API layer maps these onto gRPC status codes
// (NotFound → codes.NotFound, AlreadyExists → codes.AlreadyExists,
// Conflict → codes.Aborted).
var (
	// ErrNotFound is returned when no resource matches a Key.
	ErrNotFound = errors.New("store: resource not found")
	// ErrAlreadyExists is returned by Create when the Key is taken.
	ErrAlreadyExists = errors.New("store: resource already exists")
	// ErrConflict is returned by Update when the supplied ResourceVersion does
	// not match the stored one (optimistic-concurrency failure).
	ErrConflict = errors.New("store: resource version conflict")
)

// Key is the natural identifier of a resource, mirroring a K8s object
// reference. Project is "" for tenant-global kinds (e.g. Project itself).
type Key struct {
	TenantID string
	Kind     string
	Project  string
	Name     string
}

// Record is one stored resource. Data is the marshaled protobuf body; the
// remaining fields are the indexed metadata the store reads and writes.
type Record struct {
	Key

	// UID is assigned by the store on Create and immutable thereafter.
	UID string
	// ResourceVersion is the optimistic-concurrency token, assigned and bumped
	// by the store. Decimal string form of the stored monotonic counter.
	ResourceVersion string
	// Generation is owned by the caller; the store persists it verbatim.
	Generation int64

	Labels    map[string]string
	CreatedAt time.Time
	Data      []byte
}

// ListOptions filters a List call. An empty Project lists across all projects
// in the tenant; a non-empty Project restricts to it. LabelSelector matches by
// equality — every key/value must be present (Phase 0 keeps it simple).
type ListOptions struct {
	TenantID      string
	Kind          string
	Project       string
	LabelSelector map[string]string

	// Limit caps the page size. 0 means no limit.
	Limit int
	// Continue is the token returned by a previous List call.
	Continue string
}

// Store is the persistence seam for control-plane resources.
// Implementations must be safe for concurrent use.
type Store interface {
	// Create inserts rec, assigning UID and ResourceVersion. It returns
	// ErrAlreadyExists if the Key is taken.
	Create(ctx context.Context, rec Record) (Record, error)

	// Get returns the resource at key, or ErrNotFound.
	Get(ctx context.Context, key Key) (Record, error)

	// List returns resources matching opts, ordered by (project, name), plus a
	// continue token for the next page ("" when the page is the last).
	List(ctx context.Context, opts ListOptions) ([]Record, string, error)

	// Update replaces the resource at rec.Key. rec.ResourceVersion must equal
	// the stored value or Update returns ErrConflict; it returns ErrNotFound if
	// the resource is gone. The returned Record carries the bumped version.
	Update(ctx context.Context, rec Record) (Record, error)

	// Delete removes the resource at key and returns its last state, or
	// ErrNotFound. Hard delete; finalizer-aware soft deletion is modeled by the
	// caller via Update (setting deletion_timestamp in the body).
	Delete(ctx context.Context, key Key) (Record, error)

	// Close releases the underlying resources.
	Close() error
}
