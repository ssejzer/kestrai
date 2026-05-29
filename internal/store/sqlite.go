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
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite" // registers the pure-Go "sqlite" database/sql driver
)

// SQLite is the dev/local reference Store, backed by a single SQLite database.
type SQLite struct {
	db *sql.DB
}

var _ Store = (*SQLite)(nil)

// Open opens (creating if absent) the SQLite database at dsn, applies
// migrations, and returns a ready Store. dsn is a database/sql data source,
// e.g. "file:/var/lib/kestrai/state.db" or "file:mem.db?mode=memory&cache=shared".
func Open(ctx context.Context, dsn string) (*SQLite, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open sqlite: %w", err)
	}

	// SQLite serializes writers; a single connection sidesteps "database is
	// locked" without sprinkling retries. Fine for the embedded dev store.
	db.SetMaxOpenConns(1)

	for _, pragma := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
	} {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("store: %q: %w", pragma, err)
		}
	}

	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	return &SQLite{db: db}, nil
}

// Close implements Store.
func (s *SQLite) Close() error { return s.db.Close() }

// Create implements Store.
func (s *SQLite) Create(ctx context.Context, rec Record) (Record, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Record{}, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	var exists int
	err = tx.QueryRowContext(ctx,
		`SELECT 1 FROM resources WHERE tenant_id=? AND kind=? AND project=? AND name=?`,
		rec.TenantID, rec.Kind, rec.Project, rec.Name).Scan(&exists)
	switch {
	case err == nil:
		return Record{}, ErrAlreadyExists
	case !errors.Is(err, sql.ErrNoRows):
		return Record{}, err
	}

	rec.UID = newUID()
	const initialVersion = 1
	rec.ResourceVersion = strconv.Itoa(initialVersion)
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now().UTC()
	}

	labels, err := marshalLabels(rec.Labels)
	if err != nil {
		return Record{}, err
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO resources
			(tenant_id, kind, project, name, uid, resource_version, generation, labels, created_at, data)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.TenantID, rec.Kind, rec.Project, rec.Name, rec.UID,
		initialVersion, rec.Generation, labels, rec.CreatedAt.Format(time.RFC3339Nano), rec.Data,
	); err != nil {
		return Record{}, fmt.Errorf("store: insert: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Record{}, err
	}
	return rec, nil
}

// Get implements Store.
func (s *SQLite) Get(ctx context.Context, key Key) (Record, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT uid, resource_version, generation, labels, created_at, data
		   FROM resources WHERE tenant_id=? AND kind=? AND project=? AND name=?`,
		key.TenantID, key.Kind, key.Project, key.Name)
	rec, err := scanRecord(key, row)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, ErrNotFound
	}
	return rec, err
}

// Update implements Store.
func (s *SQLite) Update(ctx context.Context, rec Record) (Record, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Record{}, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	var (
		uid       string
		storedRV  int64
		createdAt string
	)
	err = tx.QueryRowContext(ctx,
		`SELECT uid, resource_version, created_at FROM resources
		   WHERE tenant_id=? AND kind=? AND project=? AND name=?`,
		rec.TenantID, rec.Kind, rec.Project, rec.Name).Scan(&uid, &storedRV, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, ErrNotFound
	} else if err != nil {
		return Record{}, err
	}

	if rec.ResourceVersion != strconv.FormatInt(storedRV, 10) {
		return Record{}, ErrConflict
	}

	labels, err := marshalLabels(rec.Labels)
	if err != nil {
		return Record{}, err
	}

	newRV := storedRV + 1
	if _, err := tx.ExecContext(ctx,
		`UPDATE resources SET resource_version=?, generation=?, labels=?, data=?
		   WHERE tenant_id=? AND kind=? AND project=? AND name=?`,
		newRV, rec.Generation, labels, rec.Data,
		rec.TenantID, rec.Kind, rec.Project, rec.Name,
	); err != nil {
		return Record{}, fmt.Errorf("store: update: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Record{}, err
	}

	rec.UID = uid
	rec.ResourceVersion = strconv.FormatInt(newRV, 10)
	if t, perr := time.Parse(time.RFC3339Nano, createdAt); perr == nil {
		rec.CreatedAt = t
	}
	return rec, nil
}

// Delete implements Store.
func (s *SQLite) Delete(ctx context.Context, key Key) (Record, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Record{}, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	row := tx.QueryRowContext(ctx,
		`SELECT uid, resource_version, generation, labels, created_at, data
		   FROM resources WHERE tenant_id=? AND kind=? AND project=? AND name=?`,
		key.TenantID, key.Kind, key.Project, key.Name)
	rec, err := scanRecord(key, row)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, ErrNotFound
	} else if err != nil {
		return Record{}, err
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM resources WHERE tenant_id=? AND kind=? AND project=? AND name=?`,
		key.TenantID, key.Kind, key.Project, key.Name,
	); err != nil {
		return Record{}, fmt.Errorf("store: delete: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Record{}, err
	}
	return rec, nil
}

// List implements Store. It fetches the candidate rows ordered by
// (project, name), filters by label equality in memory, then applies the
// continue token and limit. In-memory paging is fine at Phase 0 scale.
func (s *SQLite) List(ctx context.Context, opts ListOptions) ([]Record, string, error) {
	query := `SELECT project, name, uid, resource_version, generation, labels, created_at, data
		    FROM resources WHERE tenant_id=? AND kind=?`
	args := []any{opts.TenantID, opts.Kind}
	if opts.Project != "" {
		query += ` AND project=?`
		args = append(args, opts.Project)
	}
	query += ` ORDER BY project, name`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("store: list: %w", err)
	}
	defer rows.Close()

	after, err := decodeContinue(opts.Continue)
	if err != nil {
		return nil, "", err
	}

	var out []Record
	for rows.Next() {
		var (
			rec       Record
			labels    string
			createdAt string
			rv        int64
		)
		rec.Key = Key{TenantID: opts.TenantID, Kind: opts.Kind}
		if err := rows.Scan(&rec.Project, &rec.Name, &rec.UID, &rv, &rec.Generation, &labels, &createdAt, &rec.Data); err != nil {
			return nil, "", err
		}
		rec.ResourceVersion = strconv.FormatInt(rv, 10)
		if err := json.Unmarshal([]byte(labels), &rec.Labels); err != nil {
			return nil, "", fmt.Errorf("store: unmarshal labels: %w", err)
		}
		if t, perr := time.Parse(time.RFC3339Nano, createdAt); perr == nil {
			rec.CreatedAt = t
		}

		if after != nil && !cursorLess(*after, cursor{rec.Project, rec.Name}) {
			continue // already returned on a prior page
		}
		if !matchesLabels(rec.Labels, opts.LabelSelector) {
			continue
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	return paginate(out, opts.Limit)
}

// --- helpers ---

type cursor struct{ project, name string }

func cursorLess(a, b cursor) bool {
	if a.project != b.project {
		return a.project < b.project
	}
	return a.name < b.name
}

// paginate trims out to limit and returns the continue token for the next page
// (empty when out is the last page).
func paginate(out []Record, limit int) ([]Record, string, error) {
	if limit <= 0 || len(out) <= limit {
		return out, "", nil
	}
	page := out[:limit]
	last := page[len(page)-1]
	return page, encodeContinue(cursor{last.Project, last.Name}), nil
}

func encodeContinue(c cursor) string {
	return base64.RawURLEncoding.EncodeToString([]byte(c.project + "\x00" + c.name))
}

func decodeContinue(token string) (*cursor, error) {
	if token == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("store: invalid continue token: %w", err)
	}
	project, name, ok := strings.Cut(string(raw), "\x00")
	if !ok {
		return nil, fmt.Errorf("store: malformed continue token")
	}
	return &cursor{project, name}, nil
}

func matchesLabels(labels, selector map[string]string) bool {
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}

func marshalLabels(labels map[string]string) (string, error) {
	if labels == nil {
		return "{}", nil
	}
	b, err := json.Marshal(labels)
	if err != nil {
		return "", fmt.Errorf("store: marshal labels: %w", err)
	}
	return string(b), nil
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanRecord scans the standard column set into a Record carrying key.
func scanRecord(key Key, row rowScanner) (Record, error) {
	var (
		rec       Record
		labels    string
		createdAt string
		rv        int64
	)
	rec.Key = key
	if err := row.Scan(&rec.UID, &rv, &rec.Generation, &labels, &createdAt, &rec.Data); err != nil {
		return Record{}, err
	}
	rec.ResourceVersion = strconv.FormatInt(rv, 10)
	if err := json.Unmarshal([]byte(labels), &rec.Labels); err != nil {
		return Record{}, fmt.Errorf("store: unmarshal labels: %w", err)
	}
	if t, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
		rec.CreatedAt = t
	}
	return rec, nil
}

// newUID returns a random RFC 4122 version-4 UUID string. Hand-rolled to keep
// the dependency surface minimal; the value is opaque to clients.
func newUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("store: read random for uid: %v", err))
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
