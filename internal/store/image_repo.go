package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	cboxErr "cbox/internal/errors"
	"cbox/internal/image"
)

type ImageRepository struct {
	db *DB
}

func NewImageRepository(db *DB) *ImageRepository {
	return &ImageRepository{db: db}
}

func (r *ImageRepository) Create(ctx context.Context, img *image.Image) error {
	manifestJSON, err := json.Marshal(img.Manifest)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
	INSERT INTO images (id, manifest_json, created_at)
	VALUES (?, ?, ?)`,
		img.ID, string(manifestJSON), img.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert image: %w", err)
	}

	now := time.Now().UTC()
	for _, tag := range img.Tags {
		_, err = tx.ExecContext(ctx, `
		INSERT OR REPLACE INTO image_tags (tag, image_id, created_at)
		VALUES (?, ?, ?)`,
			tag, img.ID, now,
		)
		if err != nil {
			return fmt.Errorf("insert image tag: %w", err)
		}
	}

	return tx.Commit()
}

func (r *ImageRepository) AddTag(ctx context.Context, imageID string, tag string) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
	INSERT OR REPLACE INTO image_tags (tag, image_id, created_at)
	VALUES (?, ?, ?)`,
		tag, imageID, now,
	)
	return err
}

func (r *ImageRepository) Get(ctx context.Context, idOrTag string) (*image.Image, error) {
	// First check by direct ID
	query := `
	SELECT id, manifest_json, created_at
	FROM images
	WHERE id = ?`

	row := r.db.QueryRowContext(ctx, query, idOrTag)

	var img image.Image
	var manifestJSON string
	err := row.Scan(&img.ID, &manifestJSON, &img.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Try by tag
			queryByTag := `
			SELECT i.id, i.manifest_json, i.created_at
			FROM images i
			JOIN image_tags t ON i.id = t.image_id
			WHERE t.tag = ?`
			rowByTag := r.db.QueryRowContext(ctx, queryByTag, idOrTag)
			if errTag := rowByTag.Scan(&img.ID, &manifestJSON, &img.CreatedAt); errTag != nil {
				if errors.Is(errTag, sql.ErrNoRows) {
					return nil, cboxErr.ErrNotFound
				}
				return nil, fmt.Errorf("scan image by tag: %w", errTag)
			}
		} else {
			return nil, fmt.Errorf("scan image: %w", err)
		}
	}

	if err := json.Unmarshal([]byte(manifestJSON), &img.Manifest); err != nil {
		return nil, fmt.Errorf("unmarshal manifest: %w", err)
	}

	// Fetch tags
	tagRows, err := r.db.QueryContext(ctx, `SELECT tag FROM image_tags WHERE image_id = ?`, img.ID)
	if err == nil {
		defer tagRows.Close()
		for tagRows.Next() {
			var tag string
			if err := tagRows.Scan(&tag); err == nil {
				img.Tags = append(img.Tags, tag)
			}
		}
	}

	return &img, nil
}

func (r *ImageRepository) List(ctx context.Context) ([]*image.Image, error) {
	query := `SELECT id, manifest_json, created_at FROM images ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}
	defer rows.Close()

	var images []*image.Image
	for rows.Next() {
		var img image.Image
		var manifestJSON string
		if err := rows.Scan(&img.ID, &manifestJSON, &img.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan image: %w", err)
		}
		_ = json.Unmarshal([]byte(manifestJSON), &img.Manifest)
		images = append(images, &img)
	}

	// Attach tags to all images
	for _, img := range images {
		tagRows, err := r.db.QueryContext(ctx, `SELECT tag FROM image_tags WHERE image_id = ?`, img.ID)
		if err == nil {
			for tagRows.Next() {
				var tag string
				if err := tagRows.Scan(&tag); err == nil {
					img.Tags = append(img.Tags, tag)
				}
			}
			tagRows.Close()
		}
	}

	return images, nil
}

func (r *ImageRepository) Delete(ctx context.Context, idOrTag string) error {
	img, err := r.Get(ctx, idOrTag)
	if err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, _ = tx.ExecContext(ctx, `DELETE FROM image_tags WHERE image_id = ?`, img.ID)
	res, err := tx.ExecContext(ctx, `DELETE FROM images WHERE id = ?`, img.ID)
	if err != nil {
		return fmt.Errorf("delete image: %w", err)
	}

	n, _ := res.RowsAffected()
	if n == 0 {
		return cboxErr.ErrNotFound
	}

	return tx.Commit()
}
