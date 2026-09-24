package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	cboxErr "cbox/internal/errors"
	"cbox/internal/volume"
)

type VolumeRepository struct {
	db *DB
}

func NewVolumeRepository(db *DB) *VolumeRepository {
	return &VolumeRepository{db: db}
}

func (r *VolumeRepository) Create(ctx context.Context, v *volume.Volume) error {
	immutableInt := 0
	if v.Immutable {
		immutableInt = 1
	}

	query := `
	INSERT INTO volumes (id, name, source, mode, immutable, manifest_hash, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?)`

	_, err := r.db.ExecContext(ctx, query,
		v.ID, v.Name, v.Source, string(v.Mode), immutableInt, v.ManifestHash, v.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert volume: %w", err)
	}
	return nil
}

func (r *VolumeRepository) Get(ctx context.Context, idOrName string) (*volume.Volume, error) {
	query := `
	SELECT id, name, source, mode, immutable, manifest_hash, created_at
	FROM volumes
	WHERE id = ? OR name = ?`

	row := r.db.QueryRowContext(ctx, query, idOrName, idOrName)

	var v volume.Volume
	var modeStr string
	var immutableInt int
	var manifestHash sql.NullString

	err := row.Scan(&v.ID, &v.Name, &v.Source, &modeStr, &immutableInt, &manifestHash, &v.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, cboxErr.ErrNotFound
		}
		return nil, fmt.Errorf("scan volume: %w", err)
	}

	v.Mode = volume.VolumeMode(modeStr)
	v.Immutable = immutableInt == 1
	if manifestHash.Valid {
		v.ManifestHash = manifestHash.String
	}

	return &v, nil
}

func (r *VolumeRepository) List(ctx context.Context) ([]*volume.Volume, error) {
	query := `
	SELECT id, name, source, mode, immutable, manifest_hash, created_at
	FROM volumes
	ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list volumes: %w", err)
	}
	defer rows.Close()

	var volumes []*volume.Volume
	for rows.Next() {
		var v volume.Volume
		var modeStr string
		var immutableInt int
		var manifestHash sql.NullString

		if err := rows.Scan(&v.ID, &v.Name, &v.Source, &modeStr, &immutableInt, &manifestHash, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan volume row: %w", err)
		}

		v.Mode = volume.VolumeMode(modeStr)
		v.Immutable = immutableInt == 1
		if manifestHash.Valid {
			v.ManifestHash = manifestHash.String
		}

		volumes = append(volumes, &v)
	}

	return volumes, nil
}

func (r *VolumeRepository) UpdateHash(ctx context.Context, id string, hash string) error {
	query := `UPDATE volumes SET manifest_hash = ? WHERE id = ?`
	res, err := r.db.ExecContext(ctx, query, hash, id)
	if err != nil {
		return fmt.Errorf("update volume hash: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return cboxErr.ErrNotFound
	}
	return nil
}

func (r *VolumeRepository) Delete(ctx context.Context, idOrName string) error {
	query := `DELETE FROM volumes WHERE id = ? OR name = ?`
	res, err := r.db.ExecContext(ctx, query, idOrName, idOrName)
	if err != nil {
		return fmt.Errorf("delete volume: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return cboxErr.ErrNotFound
	}
	return nil
}
