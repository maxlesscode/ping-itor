// Package store provides persistent storage for monitors using SQLite.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // register sqlite driver
)

const schema = `
CREATE TABLE IF NOT EXISTS monitors (
    id                   INTEGER PRIMARY KEY AUTOINCREMENT,
    name                 TEXT NOT NULL,
    url                  TEXT NOT NULL,
    is_up                INTEGER,
    last_checked         DATETIME,
    latency_ms           INTEGER,
    http_code            INTEGER,
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    alert_sent           INTEGER NOT NULL DEFAULT 0
);`

// Monitor represents a single uptime monitor and its current state.
type Monitor struct {
	ID                  int64
	Name                string
	URL                 string
	IsUp                *bool
	LastChecked         *time.Time
	LatencyMs           *int64
	HTTPCode            *int
	ConsecutiveFailures int
	AlertSent           bool
}

// UpdateStatusInput carries all fields written during a check result.
type UpdateStatusInput struct {
	IsUp                bool
	LatencyMs           int64
	HTTPCode            int
	ConsecutiveFailures int
	AlertSent           bool
}

// Store wraps an SQLite database connection.
type Store struct {
	db *sql.DB
}

// New opens the SQLite database at dbPath, runs migrations, and returns a Store.
func New(ctx context.Context, dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", dbPath, err)
	}

	// SQLite allows only one writer at a time; serialise writes via the pool.
	db.SetMaxOpenConns(1)

	if _, err = db.ExecContext(ctx, schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return &Store{db: db}, nil
}

// Close releases the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// List returns all monitors ordered by id.
func (s *Store) List(ctx context.Context) ([]Monitor, error) {
	const q = `
SELECT id, name, url, is_up, last_checked, latency_ms, http_code,
       consecutive_failures, alert_sent
FROM monitors
ORDER BY id`

	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list monitors: %w", err)
	}
	defer rows.Close()

	var monitors []Monitor
	for rows.Next() {
		m, err := scanMonitor(rows)
		if err != nil {
			return nil, fmt.Errorf("scan monitor: %w", err)
		}
		monitors = append(monitors, m)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate monitors: %w", err)
	}
	return monitors, nil
}

// Create inserts a new monitor and returns it with the generated ID.
func (s *Store) Create(ctx context.Context, name, url string) (Monitor, error) {
	const q = `
INSERT INTO monitors (name, url) VALUES (?, ?)
RETURNING id, name, url, is_up, last_checked, latency_ms, http_code,
          consecutive_failures, alert_sent`

	row := s.db.QueryRowContext(ctx, q, name, url)
	m, err := scanMonitor(row)
	if err != nil {
		return Monitor{}, fmt.Errorf("create monitor %q: %w", name, err)
	}
	return m, nil
}

// Delete removes a monitor by ID.
func (s *Store) Delete(ctx context.Context, id int64) error {
	const q = `DELETE FROM monitors WHERE id = ?`
	if _, err := s.db.ExecContext(ctx, q, id); err != nil {
		return fmt.Errorf("delete monitor %d: %w", id, err)
	}
	return nil
}

// UpdateStatus writes a check result back to the database.
func (s *Store) UpdateStatus(ctx context.Context, id int64, input UpdateStatusInput) error {
	const q = `
UPDATE monitors
SET is_up = ?, last_checked = ?, latency_ms = ?, http_code = ?,
    consecutive_failures = ?, alert_sent = ?
WHERE id = ?`

	isUp := 0
	if input.IsUp {
		isUp = 1
	}
	alertSent := 0
	if input.AlertSent {
		alertSent = 1
	}

	if _, err := s.db.ExecContext(ctx, q,
		isUp, time.Now().UTC(), input.LatencyMs, input.HTTPCode,
		input.ConsecutiveFailures, alertSent, id,
	); err != nil {
		return fmt.Errorf("update monitor %d status: %w", id, err)
	}
	return nil
}

// Get returns a single monitor by ID.
func (s *Store) Get(ctx context.Context, id int64) (Monitor, error) {
	const q = `
SELECT id, name, url, is_up, last_checked, latency_ms, http_code,
       consecutive_failures, alert_sent
FROM monitors
WHERE id = ?`

	row := s.db.QueryRowContext(ctx, q, id)
	m, err := scanMonitor(row)
	if err != nil {
		return Monitor{}, fmt.Errorf("get monitor %d: %w", id, err)
	}
	return m, nil
}

// scanner abstracts over *sql.Row and *sql.Rows so scanMonitor can handle both.
type scanner interface {
	Scan(dest ...any) error
}

func scanMonitor(s scanner) (Monitor, error) {
	var (
		m         Monitor
		isUp      sql.NullInt64
		lastCheck sql.NullTime
		latencyMs sql.NullInt64
		httpCode  sql.NullInt64
		alertSent int
	)

	err := s.Scan(
		&m.ID, &m.Name, &m.URL,
		&isUp, &lastCheck, &latencyMs, &httpCode,
		&m.ConsecutiveFailures, &alertSent,
	)
	if err != nil {
		return Monitor{}, err
	}

	if isUp.Valid {
		up := isUp.Int64 != 0
		m.IsUp = &up
	}
	if lastCheck.Valid {
		t := lastCheck.Time
		m.LastChecked = &t
	}
	if latencyMs.Valid {
		v := latencyMs.Int64
		m.LatencyMs = &v
	}
	if httpCode.Valid {
		v := int(httpCode.Int64)
		m.HTTPCode = &v
	}
	m.AlertSent = alertSent != 0

	return m, nil
}
