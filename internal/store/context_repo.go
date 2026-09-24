package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	cboxContext "cbox/internal/context"
	cboxErr "cbox/internal/errors"
)

type ContextRepository struct {
	db *DB
}

func NewContextRepository(db *DB) *ContextRepository {
	return &ContextRepository{db: db}
}

func (r *ContextRepository) Create(ctx context.Context, c *cboxContext.Context) error {
	autoScheduleInt := 0
	if c.AutoSchedule {
		autoScheduleInt = 1
	}
	isCurrentInt := 0
	if c.IsCurrent {
		isCurrentInt = 1
	}

	query := `
	INSERT INTO contexts (name, provider, profile, auto_schedule, is_current)
	VALUES (?, ?, ?, ?, ?)`

	_, err := r.db.ExecContext(ctx, query,
		c.Name, c.Provider, c.Profile, autoScheduleInt, isCurrentInt,
	)
	if err != nil {
		return fmt.Errorf("insert context: %w", err)
	}
	return nil
}

func (r *ContextRepository) Get(ctx context.Context, name string) (*cboxContext.Context, error) {
	query := `
	SELECT name, provider, profile, auto_schedule, is_current
	FROM contexts
	WHERE name = ?`

	row := r.db.QueryRowContext(ctx, query, name)

	var c cboxContext.Context
	var autoScheduleInt, isCurrentInt int

	err := row.Scan(&c.Name, &c.Provider, &c.Profile, &autoScheduleInt, &isCurrentInt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, cboxErr.ErrNotFound
		}
		return nil, fmt.Errorf("scan context: %w", err)
	}

	c.AutoSchedule = autoScheduleInt == 1
	c.IsCurrent = isCurrentInt == 1

	return &c, nil
}

func (r *ContextRepository) GetCurrent(ctx context.Context) (*cboxContext.Context, error) {
	query := `
	SELECT name, provider, profile, auto_schedule, is_current
	FROM contexts
	WHERE is_current = 1
	LIMIT 1`

	row := r.db.QueryRowContext(ctx, query)

	var c cboxContext.Context
	var autoScheduleInt, isCurrentInt int

	err := row.Scan(&c.Name, &c.Provider, &c.Profile, &autoScheduleInt, &isCurrentInt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, cboxErr.ErrNotFound
		}
		return nil, fmt.Errorf("scan current context: %w", err)
	}

	c.AutoSchedule = autoScheduleInt == 1
	c.IsCurrent = true

	return &c, nil
}

func (r *ContextRepository) SetCurrent(ctx context.Context, name string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `UPDATE contexts SET is_current = 0`)
	if err != nil {
		return err
	}

	res, err := tx.ExecContext(ctx, `UPDATE contexts SET is_current = 1 WHERE name = ?`, name)
	if err != nil {
		return err
	}

	n, _ := res.RowsAffected()
	if n == 0 {
		return cboxErr.ErrNotFound
	}

	return tx.Commit()
}

func (r *ContextRepository) List(ctx context.Context) ([]*cboxContext.Context, error) {
	query := `
	SELECT name, provider, profile, auto_schedule, is_current
	FROM contexts
	ORDER BY name ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list contexts: %w", err)
	}
	defer rows.Close()

	var list []*cboxContext.Context
	for rows.Next() {
		var c cboxContext.Context
		var autoScheduleInt, isCurrentInt int

		if err := rows.Scan(&c.Name, &c.Provider, &c.Profile, &autoScheduleInt, &isCurrentInt); err != nil {
			return nil, fmt.Errorf("scan context row: %w", err)
		}

		c.AutoSchedule = autoScheduleInt == 1
		c.IsCurrent = isCurrentInt == 1

		list = append(list, &c)
	}

	return list, nil
}

func (r *ContextRepository) Delete(ctx context.Context, name string) error {
	query := `DELETE FROM contexts WHERE name = ?`
	res, err := r.db.ExecContext(ctx, query, name)
	if err != nil {
		return fmt.Errorf("delete context: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return cboxErr.ErrNotFound
	}
	return nil
}
