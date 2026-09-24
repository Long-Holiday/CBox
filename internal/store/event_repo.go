package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Event struct {
	ID         string    `json:"id"`
	ObjectType string    `json:"object_type"`
	ObjectID   string    `json:"object_id"`
	Type       string    `json:"type"`
	Timestamp  time.Time `json:"timestamp"`
	Payload    string    `json:"payload"`
}

type EventRepository struct {
	db *DB
}

func NewEventRepository(db *DB) *EventRepository {
	return &EventRepository{db: db}
}

func (r *EventRepository) Record(ctx context.Context, objType, objID, eventType string, payload any) error {
	var payloadStr string
	if payload != nil {
		if s, ok := payload.(string); ok {
			payloadStr = s
		} else {
			b, err := json.Marshal(payload)
			if err == nil {
				payloadStr = string(b)
			}
		}
	}

	id := uuid.New().String()
	now := time.Now().UTC()

	query := `
	INSERT INTO events (id, object_type, object_id, type, timestamp, payload)
	VALUES (?, ?, ?, ?, ?, ?)`

	_, err := r.db.ExecContext(ctx, query, id, objType, objID, eventType, now, payloadStr)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

func (r *EventRepository) ListByObject(ctx context.Context, objType, objID string) ([]*Event, error) {
	query := `
	SELECT id, object_type, object_id, type, timestamp, payload
	FROM events
	WHERE object_type = ? AND object_id = ?
	ORDER BY timestamp ASC`

	rows, err := r.db.QueryContext(ctx, query, objType, objID)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.ObjectType, &e.ObjectID, &e.Type, &e.Timestamp, &e.Payload); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, &e)
	}

	return events, nil
}

func (r *EventRepository) List(ctx context.Context, limit int) ([]*Event, error) {
	if limit <= 0 {
		limit = 100
	}
	query := `
	SELECT id, object_type, object_id, type, timestamp, payload
	FROM events
	ORDER BY timestamp DESC
	LIMIT ?`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.ObjectType, &e.ObjectID, &e.Type, &e.Timestamp, &e.Payload); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, &e)
	}

	return events, nil
}
