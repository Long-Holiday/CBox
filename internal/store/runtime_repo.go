package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	cboxErr "cbox/internal/errors"
	"cbox/internal/runtime"
)

type RuntimeRepository struct {
	db *DB
}

func NewRuntimeRepository(db *DB) *RuntimeRepository {
	return &RuntimeRepository{db: db}
}

func (r *RuntimeRepository) Create(ctx context.Context, rt *runtime.Runtime) error {
	cacheJSON, err := json.Marshal(rt.Cache)
	if err != nil {
		return fmt.Errorf("marshal cache: %w", err)
	}

	query := `
	INSERT INTO runtimes (
		id, provider, session, profile, requested_gpu, actual_gpu, state,
		created_at, last_seen, cache_json
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = r.db.ExecContext(ctx, query,
		rt.ID, rt.Provider, rt.Session, rt.Profile, rt.RequestedGPU, rt.ActualGPU,
		string(rt.State), rt.CreatedAt, rt.LastSeen, string(cacheJSON),
	)
	if err != nil {
		return fmt.Errorf("insert runtime: %w", err)
	}
	return nil
}

func (r *RuntimeRepository) Get(ctx context.Context, id string) (*runtime.Runtime, error) {
	query := `
	SELECT id, provider, session, profile, requested_gpu, actual_gpu, state,
	       created_at, last_seen, cache_json
	FROM runtimes
	WHERE id = ?`

	row := r.db.QueryRowContext(ctx, query, id)

	var rt runtime.Runtime
	var stateStr, cacheJSON string

	err := row.Scan(
		&rt.ID, &rt.Provider, &rt.Session, &rt.Profile, &rt.RequestedGPU,
		&rt.ActualGPU, &stateStr, &rt.CreatedAt, &rt.LastSeen, &cacheJSON,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, cboxErr.ErrNotFound
		}
		return nil, fmt.Errorf("scan runtime: %w", err)
	}

	rt.State = runtime.RuntimeState(stateStr)
	_ = json.Unmarshal([]byte(cacheJSON), &rt.Cache)
	if rt.Cache.Images == nil {
		rt.Cache.Images = make(map[string]bool)
	}
	if rt.Cache.Volumes == nil {
		rt.Cache.Volumes = make(map[string]string)
	}

	return &rt, nil
}

func (r *RuntimeRepository) UpdateState(ctx context.Context, id string, state runtime.RuntimeState) error {
	query := `UPDATE runtimes SET state = ?, last_seen = ? WHERE id = ?`
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, query, string(state), now, id)
	if err != nil {
		return fmt.Errorf("update runtime state: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return cboxErr.ErrNotFound
	}
	return nil
}

func (r *RuntimeRepository) UpdateCache(ctx context.Context, id string, cache runtime.RuntimeCache) error {
	cacheJSON, err := json.Marshal(cache)
	if err != nil {
		return fmt.Errorf("marshal cache: %w", err)
	}
	query := `UPDATE runtimes SET cache_json = ?, last_seen = ? WHERE id = ?`
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, query, string(cacheJSON), now, id)
	if err != nil {
		return fmt.Errorf("update runtime cache: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return cboxErr.ErrNotFound
	}
	return nil
}

func (r *RuntimeRepository) UpdateLastSeen(ctx context.Context, id string) error {
	query := `UPDATE runtimes SET last_seen = ? WHERE id = ?`
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, query, now, id)
	return err
}

func (r *RuntimeRepository) List(ctx context.Context) ([]*runtime.Runtime, error) {
	query := `
	SELECT id, provider, session, profile, requested_gpu, actual_gpu, state,
	       created_at, last_seen, cache_json
	FROM runtimes
	ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list runtimes: %w", err)
	}
	defer rows.Close()

	var runtimes []*runtime.Runtime
	for rows.Next() {
		var rt runtime.Runtime
		var stateStr, cacheJSON string

		err := rows.Scan(
			&rt.ID, &rt.Provider, &rt.Session, &rt.Profile, &rt.RequestedGPU,
			&rt.ActualGPU, &stateStr, &rt.CreatedAt, &rt.LastSeen, &cacheJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("scan runtime row: %w", err)
		}

		rt.State = runtime.RuntimeState(stateStr)
		_ = json.Unmarshal([]byte(cacheJSON), &rt.Cache)
		if rt.Cache.Images == nil {
			rt.Cache.Images = make(map[string]bool)
		}
		if rt.Cache.Volumes == nil {
			rt.Cache.Volumes = make(map[string]string)
		}

		runtimes = append(runtimes, &rt)
	}

	return runtimes, nil
}

func (r *RuntimeRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM runtimes WHERE id = ?`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete runtime: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return cboxErr.ErrNotFound
	}
	return nil
}
