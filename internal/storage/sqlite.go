package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("not found")

const (
	JobStatusQueued  = "queued"
	JobStatusRunning = "running"
	JobStatusDone    = "done"
	JobStatusFailed  = "failed"
)

type DB struct {
	path string
	conn *sql.DB
}

type Job struct {
	ID             string
	Status         string
	PromptRaw      string
	PromptFinal    string
	NegativePrompt string
	ComfyPromptID  string
	OutputFile     string
	Error          string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func Open(ctx context.Context, dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	conn.SetConnMaxLifetime(0)

	if _, err := conn.ExecContext(ctx, `PRAGMA journal_mode=WAL;`); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("set journal_mode: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys=ON;`); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("set foreign_keys: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `PRAGMA busy_timeout=5000;`); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("set busy_timeout: %w", err)
	}
	if err := runMigrations(ctx, conn); err != nil {
		_ = conn.Close()
		return nil, err
	}

	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := conn.PingContext(pingCtx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	db := &DB{path: dbPath, conn: conn}
	if err := db.ensureDefaultSettings(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return db, nil
}

func (d *DB) Close() error {
	if d == nil || d.conn == nil {
		return nil
	}
	return d.conn.Close()
}

func (d *DB) Path() string {
	if d == nil {
		return ""
	}
	return d.path
}

func (d *DB) Health(ctx context.Context) error {
	if d == nil || d.conn == nil {
		return fmt.Errorf("db not initialized")
	}
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return d.conn.PingContext(pingCtx)
}

func formatDBTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseDBTime(raw string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

func (d *DB) ensureDefaultSettings(ctx context.Context) error {
	defaults := map[string]string{
		"positive_tags":     "best quality, masterpiece, absurdres",
		"negative_tags":     "worst quality, low quality, blurry, text, watermark",
		"last_aspect_ratio": "square",
	}
	return d.UpsertSettings(ctx, defaults)
}

func (d *DB) Settings(ctx context.Context) (map[string]string, error) {
	rows, err := d.conn.QueryContext(ctx, `SELECT key, value FROM settings ORDER BY key ASC`)
	if err != nil {
		return nil, fmt.Errorf("settings query: %w", err)
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var key string
		var value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("settings scan: %w", err)
		}
		out[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("settings rows: %w", err)
	}
	return out, nil
}

func (d *DB) UpsertSettings(ctx context.Context, values map[string]string) error {
	if len(values) == 0 {
		return nil
	}
	tx, err := d.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("upsert settings begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO settings (key, value, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`)
	if err != nil {
		return fmt.Errorf("upsert settings prepare: %w", err)
	}
	defer stmt.Close()

	now := formatDBTime(time.Now())
	for key, value := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			return fmt.Errorf("upsert settings key empty")
		}
		if _, err := stmt.ExecContext(ctx, key, strings.TrimSpace(value), now); err != nil {
			return fmt.Errorf("upsert setting %q: %w", key, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("upsert settings commit: %w", err)
	}
	return nil
}

func (d *DB) CreateJob(ctx context.Context, job Job) (Job, error) {
	now := time.Now().UTC()
	job.CreatedAt = now
	job.UpdatedAt = now
	if strings.TrimSpace(job.Status) == "" {
		job.Status = JobStatusQueued
	}
	if _, err := d.conn.ExecContext(ctx, `
		INSERT INTO jobs (
			id, status, prompt_raw, prompt_final, negative_prompt,
			comfy_prompt_id, output_file, error, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		job.ID,
		job.Status,
		job.PromptRaw,
		job.PromptFinal,
		job.NegativePrompt,
		strings.TrimSpace(job.ComfyPromptID),
		strings.TrimSpace(job.OutputFile),
		strings.TrimSpace(job.Error),
		formatDBTime(job.CreatedAt),
		formatDBTime(job.UpdatedAt),
	); err != nil {
		return Job{}, fmt.Errorf("create job: %w", err)
	}
	return job, nil
}

func (d *DB) GetJob(ctx context.Context, id string) (Job, error) {
	var row Job
	var createdAt string
	var updatedAt string
	err := d.conn.QueryRowContext(ctx, `
		SELECT id, status, prompt_raw, prompt_final, negative_prompt, comfy_prompt_id, output_file, error, created_at, updated_at
		FROM jobs WHERE id = ?`, strings.TrimSpace(id)).Scan(
		&row.ID,
		&row.Status,
		&row.PromptRaw,
		&row.PromptFinal,
		&row.NegativePrompt,
		&row.ComfyPromptID,
		&row.OutputFile,
		&row.Error,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Job{}, ErrNotFound
		}
		return Job{}, fmt.Errorf("get job: %w", err)
	}
	row.CreatedAt = parseDBTime(createdAt)
	row.UpdatedAt = parseDBTime(updatedAt)
	return row, nil
}

func (d *DB) UpdateJobRunning(ctx context.Context, id string) error {
	_, err := d.conn.ExecContext(ctx, `
		UPDATE jobs SET status=?, updated_at=? WHERE id=?`,
		JobStatusRunning,
		formatDBTime(time.Now()),
		strings.TrimSpace(id),
	)
	if err != nil {
		return fmt.Errorf("update job running: %w", err)
	}
	return nil
}

func (d *DB) UpdateJobComfyPromptID(ctx context.Context, id, promptID string) error {
	_, err := d.conn.ExecContext(ctx, `
		UPDATE jobs SET comfy_prompt_id=?, updated_at=? WHERE id=?`,
		strings.TrimSpace(promptID),
		formatDBTime(time.Now()),
		strings.TrimSpace(id),
	)
	if err != nil {
		return fmt.Errorf("update job comfy prompt id: %w", err)
	}
	return nil
}

func (d *DB) UpdateJobDone(ctx context.Context, id, outputFile string) error {
	_, err := d.conn.ExecContext(ctx, `
		UPDATE jobs SET status=?, output_file=?, error='', updated_at=? WHERE id=?`,
		JobStatusDone,
		strings.TrimSpace(outputFile),
		formatDBTime(time.Now()),
		strings.TrimSpace(id),
	)
	if err != nil {
		return fmt.Errorf("update job done: %w", err)
	}
	return nil
}

func (d *DB) UpdateJobFailed(ctx context.Context, id, message string) error {
	_, err := d.conn.ExecContext(ctx, `
		UPDATE jobs SET status=?, error=?, updated_at=? WHERE id=?`,
		JobStatusFailed,
		strings.TrimSpace(message),
		formatDBTime(time.Now()),
		strings.TrimSpace(id),
	)
	if err != nil {
		return fmt.Errorf("update job failed: %w", err)
	}
	return nil
}
