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

package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func openTestStore(t *testing.T) *SQLite {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "state.db")
	s, err := Open(context.Background(), dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func projectRecord(name string, labels map[string]string) Record {
	return Record{
		Key:        Key{TenantID: "default", Kind: "Project", Name: name},
		Generation: 1,
		Labels:     labels,
		Data:       []byte("data-" + name),
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "state.db")
	s1, err := Open(context.Background(), dsn)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	s1.Close()
	// Re-opening runs migrate again; already-applied migrations must be skipped.
	s2, err := Open(context.Background(), dsn)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	s2.Close()
}

func TestCreateGetRoundTrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	created, err := s.Create(ctx, projectRecord("alpha", map[string]string{"team": "core"}))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.UID == "" {
		t.Error("Create did not assign a UID")
	}
	if created.ResourceVersion != "1" {
		t.Errorf("ResourceVersion = %q, want \"1\"", created.ResourceVersion)
	}

	got, err := s.Get(ctx, created.Key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.UID != created.UID {
		t.Errorf("UID = %q, want %q", got.UID, created.UID)
	}
	if string(got.Data) != "data-alpha" {
		t.Errorf("Data = %q, want %q", got.Data, "data-alpha")
	}
	if got.Labels["team"] != "core" {
		t.Errorf("Labels = %v, want team=core", got.Labels)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
}

func TestCreateDuplicateIsAlreadyExists(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	if _, err := s.Create(ctx, projectRecord("dup", nil)); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	if _, err := s.Create(ctx, projectRecord("dup", nil)); !errors.Is(err, ErrAlreadyExists) {
		t.Errorf("second Create err = %v, want ErrAlreadyExists", err)
	}
}

func TestCreateUnknownTenantFails(t *testing.T) {
	s := openTestStore(t)
	rec := projectRecord("orphan", nil)
	rec.TenantID = "ghost" // not seeded; foreign key must reject
	if _, err := s.Create(context.Background(), rec); err == nil {
		t.Fatal("Create with unknown tenant succeeded, want foreign-key error")
	}
}

func TestGetMissingIsNotFound(t *testing.T) {
	s := openTestStore(t)
	_, err := s.Get(context.Background(), Key{TenantID: "default", Kind: "Project", Name: "nope"})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateOptimisticConcurrency(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	created, err := s.Create(ctx, projectRecord("beta", nil))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	created.Data = []byte("updated")
	created.Generation = 2
	updated, err := s.Update(ctx, created)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.ResourceVersion != "2" {
		t.Errorf("ResourceVersion = %q, want \"2\"", updated.ResourceVersion)
	}
	if updated.UID != created.UID {
		t.Errorf("UID changed across update: %q -> %q", created.UID, updated.UID)
	}

	// Re-using the now-stale version must conflict.
	created.ResourceVersion = "1"
	if _, err := s.Update(ctx, created); !errors.Is(err, ErrConflict) {
		t.Errorf("stale Update err = %v, want ErrConflict", err)
	}
}

func TestUpdateMissingIsNotFound(t *testing.T) {
	s := openTestStore(t)
	rec := projectRecord("ghost", nil)
	rec.ResourceVersion = "1"
	if _, err := s.Update(context.Background(), rec); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestDelete(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	created, err := s.Create(ctx, projectRecord("gamma", nil))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	deleted, err := s.Delete(ctx, created.Key)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if deleted.UID != created.UID {
		t.Errorf("deleted UID = %q, want %q", deleted.UID, created.UID)
	}
	if _, err := s.Get(ctx, created.Key); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after Delete err = %v, want ErrNotFound", err)
	}
	if _, err := s.Delete(ctx, created.Key); !errors.Is(err, ErrNotFound) {
		t.Errorf("second Delete err = %v, want ErrNotFound", err)
	}
}

func TestListFiltersAndPaginates(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	for _, n := range []string{"a", "b", "c"} {
		if _, err := s.Create(ctx, projectRecord(n, map[string]string{"team": "core"})); err != nil {
			t.Fatalf("Create %s: %v", n, err)
		}
	}
	if _, err := s.Create(ctx, projectRecord("d", map[string]string{"team": "ext"})); err != nil {
		t.Fatalf("Create d: %v", err)
	}

	t.Run("label selector", func(t *testing.T) {
		got, _, err := s.List(ctx, ListOptions{TenantID: "default", Kind: "Project", LabelSelector: map[string]string{"team": "ext"}})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(got) != 1 || got[0].Name != "d" {
			t.Errorf("got %d records (want 1: d): %+v", len(got), got)
		}
	})

	t.Run("pagination", func(t *testing.T) {
		page1, token, err := s.List(ctx, ListOptions{TenantID: "default", Kind: "Project", Limit: 2})
		if err != nil {
			t.Fatalf("List page1: %v", err)
		}
		if len(page1) != 2 || page1[0].Name != "a" || page1[1].Name != "b" {
			t.Fatalf("page1 = %v, want [a b]", names(page1))
		}
		if token == "" {
			t.Fatal("expected a continue token after page1")
		}

		page2, token2, err := s.List(ctx, ListOptions{TenantID: "default", Kind: "Project", Limit: 2, Continue: token})
		if err != nil {
			t.Fatalf("List page2: %v", err)
		}
		if got := names(page2); len(got) != 2 || got[0] != "c" || got[1] != "d" {
			t.Fatalf("page2 = %v, want [c d]", got)
		}
		if token2 != "" {
			t.Errorf("expected no token after last page, got %q", token2)
		}
	})
}

func TestListKindIsolation(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if _, err := s.Create(ctx, projectRecord("shared", nil)); err != nil {
		t.Fatalf("Create Project: %v", err)
	}
	wf := Record{Key: Key{TenantID: "default", Kind: "Workflow", Project: "p1", Name: "shared"}, Data: []byte("wf")}
	if _, err := s.Create(ctx, wf); err != nil {
		t.Fatalf("Create Workflow: %v", err)
	}

	got, _, err := s.List(ctx, ListOptions{TenantID: "default", Kind: "Project"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("Project list returned %d, want 1 (Workflow must not leak)", len(got))
	}
}

func names(recs []Record) []string {
	out := make([]string, len(recs))
	for i, r := range recs {
		out[i] = r.Name
	}
	return out
}
