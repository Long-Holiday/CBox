package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"cbox/internal/container"
	cboxErr "cbox/internal/errors"
)

type ContainerRepository struct {
	db *DB
}

func NewContainerRepository(db *DB) *ContainerRepository {
	return &ContainerRepository{db: db}
}

func (r *ContainerRepository) Create(ctx context.Context, c *container.Container) error {
	cmdJSON, err := json.Marshal(c.Command)
	if err != nil {
		return fmt.Errorf("marshal command: %w", err)
	}

	envJSON, err := json.Marshal(c.Env)
	if err != nil {
		return fmt.Errorf("marshal env: %w", err)
	}

	resourceJSON, err := json.Marshal(c.Resource)
	if err != nil {
		return fmt.Errorf("marshal resource: %w", err)
	}

	resumeJSON, err := json.Marshal(c.ResumeCommand)
	if err != nil {
		return fmt.Errorf("marshal resume command: %w", err)
	}

	secretsJSON, err := json.Marshal(c.Secrets)
	if err != nil {
		return fmt.Errorf("marshal secrets: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	query := `
	INSERT INTO containers (
		id, name, image_id, state, desired_state, command_json, env_json,
		workdir, resource_json, restart_policy, resume_command_json, secrets_json,
		runtime_id, exit_code, created_at, started_at, finished_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = tx.ExecContext(ctx, query,
		c.ID, c.Name, c.ImageID, string(c.State), string(c.DesiredState),
		string(cmdJSON), string(envJSON), c.WorkDir, string(resourceJSON),
		string(c.RestartPolicy), string(resumeJSON), string(secretsJSON),
		c.RuntimeID, c.ExitCode, c.CreatedAt, c.StartedAt, c.FinishedAt,
	)
	if err != nil {
		return fmt.Errorf("insert container: %w", err)
	}

	for _, m := range c.Mounts {
		_, err = tx.ExecContext(ctx, `
		INSERT INTO container_mounts (container_id, volume_id, source, target, mode)
		VALUES (?, ?, ?, ?, ?)`,
			c.ID, m.VolumeID, m.Source, m.Target, m.Mode,
		)
		if err != nil {
			return fmt.Errorf("insert mount: %w", err)
		}
	}

	return tx.Commit()
}

func (r *ContainerRepository) Get(ctx context.Context, idOrName string) (*container.Container, error) {
	query := `
	SELECT id, name, image_id, state, desired_state, command_json, env_json,
	       workdir, resource_json, restart_policy, resume_command_json, secrets_json,
	       runtime_id, exit_code, created_at, started_at, finished_at
	FROM containers
	WHERE id = ? OR name = ?`

	row := r.db.QueryRowContext(ctx, query, idOrName, idOrName)

	var c container.Container
	var stateStr, desiredStr, cmdJSON, envJSON, resJSON, resumeJSON, secretsJSON string
	var workDir string
	var runtimeID sql.NullString
	var exitCode sql.NullInt64
	var startedAt, finishedAt sql.NullTime

	err := row.Scan(
		&c.ID, &c.Name, &c.ImageID, &stateStr, &desiredStr,
		&cmdJSON, &envJSON, &workDir, &resJSON, &c.RestartPolicy,
		&resumeJSON, &secretsJSON, &runtimeID, &exitCode,
		&c.CreatedAt, &startedAt, &finishedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, cboxErr.ErrNotFound
		}
		return nil, fmt.Errorf("scan container: %w", err)
	}

	c.State = container.ContainerState(stateStr)
	c.DesiredState = container.DesiredState(desiredStr)
	c.WorkDir = workDir
	if runtimeID.Valid {
		c.RuntimeID = runtimeID.String
	}
	if exitCode.Valid {
		code := int(exitCode.Int64)
		c.ExitCode = &code
	}
	if startedAt.Valid {
		c.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		c.FinishedAt = &finishedAt.Time
	}

	_ = json.Unmarshal([]byte(cmdJSON), &c.Command)
	_ = json.Unmarshal([]byte(envJSON), &c.Env)
	_ = json.Unmarshal([]byte(resJSON), &c.Resource)
	_ = json.Unmarshal([]byte(resumeJSON), &c.ResumeCommand)
	_ = json.Unmarshal([]byte(secretsJSON), &c.Secrets)

	// Fetch mounts
	mountRows, err := r.db.QueryContext(ctx, `
	SELECT volume_id, source, target, mode
	FROM container_mounts
	WHERE container_id = ?`, c.ID)
	if err != nil {
		return nil, fmt.Errorf("query mounts: %w", err)
	}
	defer mountRows.Close()

	for mountRows.Next() {
		var m container.Mount
		if err := mountRows.Scan(&m.VolumeID, &m.Source, &m.Target, &m.Mode); err != nil {
			return nil, fmt.Errorf("scan mount: %w", err)
		}
		c.Mounts = append(c.Mounts, m)
	}

	return &c, nil
}

func (r *ContainerRepository) Update(ctx context.Context, c *container.Container) error {
	cmdJSON, _ := json.Marshal(c.Command)
	envJSON, _ := json.Marshal(c.Env)
	resourceJSON, _ := json.Marshal(c.Resource)
	resumeJSON, _ := json.Marshal(c.ResumeCommand)
	secretsJSON, _ := json.Marshal(c.Secrets)

	query := `
	UPDATE containers
	SET state = ?, desired_state = ?, command_json = ?, env_json = ?,
	    workdir = ?, resource_json = ?, restart_policy = ?, resume_command_json = ?,
	    secrets_json = ?, runtime_id = ?, exit_code = ?, started_at = ?, finished_at = ?
	WHERE id = ?`

	res, err := r.db.ExecContext(ctx, query,
		string(c.State), string(c.DesiredState), string(cmdJSON), string(envJSON),
		c.WorkDir, string(resourceJSON), string(c.RestartPolicy), string(resumeJSON),
		string(secretsJSON), c.RuntimeID, c.ExitCode, c.StartedAt, c.FinishedAt,
		c.ID,
	)
	if err != nil {
		return fmt.Errorf("update container: %w", err)
	}

	n, _ := res.RowsAffected()
	if n == 0 {
		return cboxErr.ErrNotFound
	}
	return nil
}

func (r *ContainerRepository) UpdateState(ctx context.Context, id string, state container.ContainerState) error {
	var query string
	now := time.Now().UTC()

	switch state {
	case container.StateRunning:
		query = `UPDATE containers SET state = ?, started_at = COALESCE(started_at, ?) WHERE id = ?`
		_, err := r.db.ExecContext(ctx, query, string(state), now, id)
		return err
	case container.StateExited, container.StateStopped, container.StateFailed:
		query = `UPDATE containers SET state = ?, finished_at = ? WHERE id = ?`
		_, err := r.db.ExecContext(ctx, query, string(state), now, id)
		return err
	default:
		query = `UPDATE containers SET state = ? WHERE id = ?`
		_, err := r.db.ExecContext(ctx, query, string(state), id)
		return err
	}
}

func (r *ContainerRepository) UpdateRuntime(ctx context.Context, id string, runtimeID string) error {
	query := `UPDATE containers SET runtime_id = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, runtimeID, id)
	return err
}

func (r *ContainerRepository) UpdateExitCode(ctx context.Context, id string, exitCode int) error {
	query := `UPDATE containers SET exit_code = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, exitCode, id)
	return err
}

func (r *ContainerRepository) List(ctx context.Context) ([]*container.Container, error) {
	query := `
	SELECT id, name, image_id, state, desired_state, command_json, env_json,
	       workdir, resource_json, restart_policy, resume_command_json, secrets_json,
	       runtime_id, exit_code, created_at, started_at, finished_at
	FROM containers
	ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}
	defer rows.Close()

	var containers []*container.Container
	for rows.Next() {
		var c container.Container
		var stateStr, desiredStr, cmdJSON, envJSON, resJSON, resumeJSON, secretsJSON string
		var workDir string
		var runtimeID sql.NullString
		var exitCode sql.NullInt64
		var startedAt, finishedAt sql.NullTime

		err := rows.Scan(
			&c.ID, &c.Name, &c.ImageID, &stateStr, &desiredStr,
			&cmdJSON, &envJSON, &workDir, &resJSON, &c.RestartPolicy,
			&resumeJSON, &secretsJSON, &runtimeID, &exitCode,
			&c.CreatedAt, &startedAt, &finishedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan container row: %w", err)
		}

		c.State = container.ContainerState(stateStr)
		c.DesiredState = container.DesiredState(desiredStr)
		c.WorkDir = workDir
		if runtimeID.Valid {
			c.RuntimeID = runtimeID.String
		}
		if exitCode.Valid {
			code := int(exitCode.Int64)
			c.ExitCode = &code
		}
		if startedAt.Valid {
			c.StartedAt = &startedAt.Time
		}
		if finishedAt.Valid {
			c.FinishedAt = &finishedAt.Time
		}

		_ = json.Unmarshal([]byte(cmdJSON), &c.Command)
		_ = json.Unmarshal([]byte(envJSON), &c.Env)
		_ = json.Unmarshal([]byte(resJSON), &c.Resource)
		_ = json.Unmarshal([]byte(resumeJSON), &c.ResumeCommand)
		_ = json.Unmarshal([]byte(secretsJSON), &c.Secrets)

		containers = append(containers, &c)
	}

	return containers, nil
}

func (r *ContainerRepository) Delete(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, _ = tx.ExecContext(ctx, `DELETE FROM container_mounts WHERE container_id = ?`, id)
	res, err := tx.ExecContext(ctx, `DELETE FROM containers WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete container: %w", err)
	}

	n, _ := res.RowsAffected()
	if n == 0 {
		return cboxErr.ErrNotFound
	}

	return tx.Commit()
}
